package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// ChunkMatch is a retrieval result combining a code chunk with its relevance score.
type ChunkMatch struct {
	ID         int64
	Repo       string
	FilePath   string
	ChunkType  string
	SymbolName string
	StartLine  int
	EndLine    int
	Content    string
	Score      float64 // combined relevance score (higher = more relevant)
}

// Retriever performs hybrid search (FTS5 keyword + vector cosine similarity)
// over the codebase index, merging results with Reciprocal Rank Fusion.
type Retriever struct {
	db       *sql.DB
	embedder Embedder
}

// NewRetriever creates a new Retriever.
func NewRetriever(db *sql.DB, embedder Embedder) *Retriever {
	return &Retriever{db: db, embedder: embedder}
}

// Search performs hybrid search for the given query against the specified repo.
// Returns up to topK results ranked by combined relevance.
func (r *Retriever) Search(ctx context.Context, repo, query string, topK int) ([]ChunkMatch, error) {
	if topK <= 0 {
		topK = 10
	}
	fetchK := topK * 3 // fetch more than needed for better RRF quality

	// Run keyword and vector searches.
	keywordResults, err := r.ftsSearch(repo, query, fetchK)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}

	var vectorResults []scoredChunk
	if r.embedder != nil {
		vectorResults, err = r.vectorSearch(ctx, repo, query, fetchK)
		if err != nil {
			// Vector search failure is non-fatal; degrade to keyword-only.
			vectorResults = nil
		}
	}

	// Merge with RRF if we have both result sets.
	if len(vectorResults) > 0 {
		merged := mergeRRF(keywordResults, vectorResults, topK)
		return merged, nil
	}

	// Keyword-only fallback.
	results := make([]ChunkMatch, 0, min(topK, len(keywordResults)))
	for i, sc := range keywordResults {
		if i >= topK {
			break
		}
		results = append(results, sc.chunk)
	}
	return results, nil
}

// SearchSymbol searches for an exact symbol name in the codebase.
func (r *Retriever) SearchSymbol(repo, symbol string, topK int) ([]ChunkMatch, error) {
	rows, err := r.db.Query(
		`SELECT id, repo, file_path, chunk_type, symbol_name, start_line, end_line, content
		 FROM codebase_chunks
		 WHERE repo = ? AND symbol_name LIKE ?
		 LIMIT ?`,
		repo, "%"+symbol+"%", topK,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunkRows(rows)
}

// --- FTS5 keyword search ---

func (r *Retriever) ftsSearch(repo, query string, limit int) ([]scoredChunk, error) {
	// Escape FTS5 special characters and build a simple query.
	ftsQuery := sanitizeFTS(query)

	rows, err := r.db.Query(
		`SELECT c.id, c.repo, c.file_path, c.chunk_type, c.symbol_name,
		        c.start_line, c.end_line, c.content, rank
		 FROM codebase_chunks_fts fts
		 JOIN codebase_chunks c ON c.id = fts.rowid
		 WHERE codebase_chunks_fts MATCH ? AND c.repo = ?
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery, repo, limit,
	)
	if err != nil {
		// FTS might not be available; fall back to LIKE search.
		return r.likeSearch(repo, query, limit)
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var sc scoredChunk
		var rank float64
		err := rows.Scan(
			&sc.chunk.ID, &sc.chunk.Repo, &sc.chunk.FilePath,
			&sc.chunk.ChunkType, &sc.chunk.SymbolName,
			&sc.chunk.StartLine, &sc.chunk.EndLine, &sc.chunk.Content,
			&rank,
		)
		if err != nil {
			continue
		}
		sc.score = -rank // FTS5 rank is negative (lower = better), flip it.
		results = append(results, sc)
	}

	if len(results) == 0 {
		return r.likeSearch(repo, query, limit)
	}
	return results, nil
}

func (r *Retriever) likeSearch(repo, query string, limit int) ([]scoredChunk, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil, nil
	}

	// Build a WHERE clause matching any word in content or symbol_name.
	var conditions []string
	var args []any
	args = append(args, repo)
	for _, w := range words {
		conditions = append(conditions, "(content LIKE ? OR symbol_name LIKE ? OR file_path LIKE ?)")
		pattern := "%" + w + "%"
		args = append(args, pattern, pattern, pattern)
	}

	q := fmt.Sprintf(
		`SELECT id, repo, file_path, chunk_type, symbol_name, start_line, end_line, content
		 FROM codebase_chunks
		 WHERE repo = ? AND (%s)
		 LIMIT ?`,
		strings.Join(conditions, " OR "),
	)
	args = append(args, limit)

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var sc scoredChunk
		err := rows.Scan(
			&sc.chunk.ID, &sc.chunk.Repo, &sc.chunk.FilePath,
			&sc.chunk.ChunkType, &sc.chunk.SymbolName,
			&sc.chunk.StartLine, &sc.chunk.EndLine, &sc.chunk.Content,
		)
		if err != nil {
			continue
		}
		// Simple relevance: count how many query words match.
		lower := strings.ToLower(sc.chunk.Content + " " + sc.chunk.SymbolName)
		for _, w := range words {
			if strings.Contains(lower, strings.ToLower(w)) {
				sc.score++
			}
		}
		results = append(results, sc)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	return results, nil
}

// --- Vector cosine similarity search ---

func (r *Retriever) vectorSearch(ctx context.Context, repo, query string, limit int) ([]scoredChunk, error) {
	queryEmb, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Load all embeddings for this repo. For repos under 100K lines
	// (~5000 chunks) this is fast enough with brute-force cosine similarity.
	rows, err := r.db.Query(
		`SELECT id, repo, file_path, chunk_type, symbol_name,
		        start_line, end_line, content, embedding
		 FROM codebase_chunks
		 WHERE repo = ? AND embedding IS NOT NULL`,
		repo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var sc scoredChunk
		var embBlob []byte
		err := rows.Scan(
			&sc.chunk.ID, &sc.chunk.Repo, &sc.chunk.FilePath,
			&sc.chunk.ChunkType, &sc.chunk.SymbolName,
			&sc.chunk.StartLine, &sc.chunk.EndLine, &sc.chunk.Content,
			&embBlob,
		)
		if err != nil {
			continue
		}
		chunkEmb := bytesToFloat64s(embBlob)
		if len(chunkEmb) == 0 {
			continue
		}
		sc.score = cosineSimilarity(queryEmb, chunkEmb)
		results = append(results, sc)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// --- Reciprocal Rank Fusion ---

type scoredChunk struct {
	chunk ChunkMatch
	score float64
}

// mergeRRF merges two ranked result lists using Reciprocal Rank Fusion.
// RRF score = sum(1 / (k + rank)) across all lists.
// k=60 is the standard constant from the original RRF paper.
func mergeRRF(listA, listB []scoredChunk, topK int) []ChunkMatch {
	lists := [][]scoredChunk{listA, listB}
	const k = 60

	type entry struct {
		chunk ChunkMatch
		score float64
	}
	scores := make(map[int64]*entry)

	for _, list := range lists {
		for rank, sc := range list {
			e, ok := scores[sc.chunk.ID]
			if !ok {
				e = &entry{chunk: sc.chunk}
				scores[sc.chunk.ID] = e
			}
			e.score += 1.0 / float64(k+rank+1)
		}
	}

	ranked := make([]entry, 0, len(scores))
	for _, e := range scores {
		ranked = append(ranked, *e)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}

	results := make([]ChunkMatch, len(ranked))
	for i, e := range ranked {
		e.chunk.Score = e.score
		results[i] = e.chunk
	}
	return results
}

// --- Context formatting ---

// FormatCodeContext builds a markdown string from retrieved chunks suitable
// for injecting into an agent prompt.
func FormatCodeContext(matches []ChunkMatch) string {
	if len(matches) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Relevant Code Context\n\n")

	seenFiles := make(map[string]bool)
	for _, m := range matches {
		if !seenFiles[m.FilePath] {
			b.WriteString(fmt.Sprintf("### %s\n", m.FilePath))
			seenFiles[m.FilePath] = true
		}
		if m.SymbolName != "" {
			b.WriteString(fmt.Sprintf("**%s** `%s` (lines %d-%d):\n",
				m.ChunkType, m.SymbolName, m.StartLine, m.EndLine))
		}
		b.WriteString("```\n")
		// Truncate very long chunks in context.
		content := m.Content
		if len(content) > 2000 {
			content = content[:2000] + "\n// ... (truncated)"
		}
		b.WriteString(content)
		b.WriteString("\n```\n\n")
	}
	return b.String()
}

// --- Helpers ---

func sanitizeFTS(query string) string {
	// Remove FTS5 special characters that could cause syntax errors.
	replacer := strings.NewReplacer(
		"*", "",
		"\"", "",
		"(", "",
		")", "",
		":", "",
		"^", "",
		"~", "",
		"+", "",
		"-", "",
	)
	cleaned := replacer.Replace(query)

	// Split into words and join with OR for broader matching.
	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return ""
	}
	// Use implicit OR by just returning space-separated terms.
	return strings.Join(words, " OR ")
}

func scanChunkRows(rows *sql.Rows) ([]ChunkMatch, error) {
	var results []ChunkMatch
	for rows.Next() {
		var m ChunkMatch
		err := rows.Scan(
			&m.ID, &m.Repo, &m.FilePath, &m.ChunkType, &m.SymbolName,
			&m.StartLine, &m.EndLine, &m.Content,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
