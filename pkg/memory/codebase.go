package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodebaseIndex manages the indexed representation of a repository's code.
// It stores code chunks, embeddings, and metadata in SQLite for retrieval.
type CodebaseIndex struct {
	db       *sql.DB
	embedder Embedder
}

// NewCodebaseIndex creates a new CodebaseIndex using the given database and embedder.
func NewCodebaseIndex(db *sql.DB, embedder Embedder) *CodebaseIndex {
	return &CodebaseIndex{db: db, embedder: embedder}
}

// IndexStats contains statistics about the current index state.
type IndexStats struct {
	Repo        string
	LastCommit  string
	LastIndexed time.Time
	TotalFiles  int
	TotalChunks int
}

// Index performs a full reindex of the repository at repoPath.
// It walks the file tree, chunks all indexable files, generates embeddings,
// and stores everything in the database.
func (c *CodebaseIndex) Index(ctx context.Context, repo, repoPath string) error {
	// Clear existing data for this repo.
	if err := c.clearRepo(repo); err != nil {
		return fmt.Errorf("clearing repo: %w", err)
	}

	var totalFiles, totalChunks int

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't read
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		if !IsIndexable(relPath) {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Skip very large files (>100KB).
		if len(source) > 100*1024 {
			return nil
		}

		hash := fileHash(source)
		lang := DetectLanguage(relPath)

		if err := c.upsertFile(repo, relPath, lang, hash); err != nil {
			return fmt.Errorf("upserting file %s: %w", relPath, err)
		}
		totalFiles++

		chunks := ChunkFile(relPath, source)
		for _, chunk := range chunks {
			if err := c.insertChunk(ctx, repo, chunk); err != nil {
				return fmt.Errorf("inserting chunk %s/%s: %w", relPath, chunk.SymbolName, err)
			}
			totalChunks++
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking repo: %w", err)
	}

	// Record the current commit.
	commit := currentCommit(repoPath)
	if err := c.updateIndexState(repo, commit, totalFiles, totalChunks); err != nil {
		return fmt.Errorf("updating index state: %w", err)
	}

	// Rebuild FTS index.
	c.rebuildFTS()

	return nil
}

// UpdateFromDiff incrementally updates the index based on files changed
// since the last indexed commit.
func (c *CodebaseIndex) UpdateFromDiff(ctx context.Context, repo, repoPath string) error {
	lastCommit, err := c.getLastCommit(repo)
	if err != nil || lastCommit == "" {
		// No previous index — do a full reindex.
		return c.Index(ctx, repo, repoPath)
	}

	headCommit := currentCommit(repoPath)
	if headCommit == lastCommit {
		return nil // no changes
	}

	changed, err := gitChangedFiles(repoPath, lastCommit)
	if err != nil {
		// If git diff fails (e.g. commit was rebased away), full reindex.
		return c.Index(ctx, repo, repoPath)
	}

	for _, change := range changed {
		// Remove old chunks for modified/deleted files.
		c.deleteFileChunks(repo, change.path)
		c.deleteFile(repo, change.path)

		if change.status == "D" {
			continue // file was deleted, nothing to re-add
		}

		fullPath := filepath.Join(repoPath, change.path)
		if !IsIndexable(change.path) {
			continue
		}

		source, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if len(source) > 100*1024 {
			continue
		}

		hash := fileHash(source)
		lang := DetectLanguage(change.path)
		c.upsertFile(repo, change.path, lang, hash)

		chunks := ChunkFile(change.path, source)
		for _, chunk := range chunks {
			c.insertChunk(ctx, repo, chunk)
		}
	}

	// Update state.
	stats, _ := c.getStats(repo)
	c.updateIndexState(repo, headCommit, stats.TotalFiles, stats.TotalChunks)
	c.rebuildFTS()

	return nil
}

// GetStats returns index statistics for the given repo.
func (c *CodebaseIndex) GetStats(repo string) (*IndexStats, error) {
	return c.getStats(repo)
}

// --- Database helpers ---

func (c *CodebaseIndex) clearRepo(repo string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM codebase_chunks WHERE repo = ?", repo)
	tx.Exec("DELETE FROM codebase_files WHERE repo = ?", repo)
	tx.Exec("DELETE FROM codebase_index_state WHERE repo = ?", repo)

	return tx.Commit()
}

func (c *CodebaseIndex) upsertFile(repo, path, lang, hash string) error {
	_, err := c.db.Exec(
		`INSERT INTO codebase_files (repo, path, language, hash, indexed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (repo, path) DO UPDATE SET
		   language = excluded.language,
		   hash = excluded.hash,
		   indexed_at = excluded.indexed_at`,
		repo, path, lang, hash, time.Now().UTC(),
	)
	return err
}

func (c *CodebaseIndex) insertChunk(ctx context.Context, repo string, chunk Chunk) error {
	var embeddingBlob []byte
	if c.embedder != nil {
		text := EmbeddableText(chunk)
		vec, err := c.embedder.Embed(ctx, text)
		if err == nil && len(vec) > 0 {
			embeddingBlob = float64sToBytes(vec)
		}
	}

	_, err := c.db.Exec(
		`INSERT INTO codebase_chunks (repo, file_path, chunk_type, symbol_name, start_line, end_line, content, embedding)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		repo, chunk.FilePath, chunk.ChunkType, chunk.SymbolName,
		chunk.StartLine, chunk.EndLine, chunk.Content, embeddingBlob,
	)
	return err
}

func (c *CodebaseIndex) deleteFileChunks(repo, path string) {
	c.db.Exec("DELETE FROM codebase_chunks WHERE repo = ? AND file_path = ?", repo, path)
}

func (c *CodebaseIndex) deleteFile(repo, path string) {
	c.db.Exec("DELETE FROM codebase_files WHERE repo = ? AND path = ?", repo, path)
}

func (c *CodebaseIndex) updateIndexState(repo, commit string, files, chunks int) error {
	_, err := c.db.Exec(
		`INSERT INTO codebase_index_state (repo, last_commit, last_indexed, total_files, total_chunks)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (repo) DO UPDATE SET
		   last_commit = excluded.last_commit,
		   last_indexed = excluded.last_indexed,
		   total_files = excluded.total_files,
		   total_chunks = excluded.total_chunks`,
		repo, commit, time.Now().UTC(), files, chunks,
	)
	return err
}

func (c *CodebaseIndex) getLastCommit(repo string) (string, error) {
	var commit string
	err := c.db.QueryRow(
		"SELECT last_commit FROM codebase_index_state WHERE repo = ?", repo,
	).Scan(&commit)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return commit, err
}

func (c *CodebaseIndex) getStats(repo string) (*IndexStats, error) {
	stats := &IndexStats{Repo: repo}

	row := c.db.QueryRow(
		"SELECT last_commit, last_indexed, total_files, total_chunks FROM codebase_index_state WHERE repo = ?", repo,
	)
	err := row.Scan(&stats.LastCommit, &stats.LastIndexed, &stats.TotalFiles, &stats.TotalChunks)
	if err == sql.ErrNoRows {
		// Count from actual tables.
		c.db.QueryRow("SELECT COUNT(*) FROM codebase_files WHERE repo = ?", repo).Scan(&stats.TotalFiles)
		c.db.QueryRow("SELECT COUNT(*) FROM codebase_chunks WHERE repo = ?", repo).Scan(&stats.TotalChunks)
		return stats, nil
	}
	return stats, err
}

func (c *CodebaseIndex) rebuildFTS() {
	// Rebuild the FTS content sync. This is fast for typical repo sizes.
	c.db.Exec("INSERT INTO codebase_chunks_fts(codebase_chunks_fts) VALUES('rebuild')")
}

// --- Git helpers ---

type fileChange struct {
	status string // "A", "M", "D", "R"
	path   string
}

func gitChangedFiles(repoPath, sinceCommit string) ([]fileChange, error) {
	cmd := exec.Command("git", "diff", "--name-status", sinceCommit, "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var changes []fileChange
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		path := parts[1]

		// Handle renames: Rxxx old_path new_path
		if strings.HasPrefix(status, "R") {
			changes = append(changes, fileChange{status: "D", path: parts[1]})
			if len(parts) > 2 {
				changes = append(changes, fileChange{status: "A", path: parts[2]})
			}
			continue
		}

		changes = append(changes, fileChange{status: status[:1], path: path})
	}
	return changes, nil
}

func currentCommit(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --- Encoding helpers ---

func fileHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

func float64sToBytes(fs []float64) []byte {
	b := make([]byte, len(fs)*8)
	for i, f := range fs {
		bits := math.Float64bits(f)
		binary.LittleEndian.PutUint64(b[i*8:], bits)
	}
	return b
}

func bytesToFloat64s(b []byte) []float64 {
	if len(b)%8 != 0 {
		return nil
	}
	fs := make([]float64, len(b)/8)
	for i := range fs {
		bits := binary.LittleEndian.Uint64(b[i*8:])
		fs[i] = math.Float64frombits(bits)
	}
	return fs
}
