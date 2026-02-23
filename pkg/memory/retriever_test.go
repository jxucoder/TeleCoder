package memory

import (
	"context"
	"testing"
)

func TestRetriever_LikeSearch(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	// Index the repo first.
	idx := NewCodebaseIndex(db, nil)
	if err := idx.Index(ctx, "test/repo", repo); err != nil {
		t.Fatalf("Index: %v", err)
	}

	ret := NewRetriever(db, nil)

	// Search for "Server" — should find Server struct, NewServer, Start, handleHealth.
	results, err := ret.Search(ctx, "test/repo", "Server", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'Server' query")
	}

	// At least one result should reference Server.
	found := false
	for _, r := range results {
		if r.SymbolName == "Server" || r.SymbolName == "NewServer" || r.SymbolName == "Server.Start" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one Server-related result")
	}
}

func TestRetriever_VectorSearch(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	// Index with embedder.
	idx := NewCodebaseIndex(db, &mockEmbedder{})
	if err := idx.Index(ctx, "test/repo", repo); err != nil {
		t.Fatalf("Index: %v", err)
	}

	ret := NewRetriever(db, &mockEmbedder{})

	// Search should use both FTS and vector search.
	results, err := ret.Search(ctx, "test/repo", "how does the server start", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for vector search query")
	}
}

func TestRetriever_SearchSymbol(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	idx := NewCodebaseIndex(db, nil)
	idx.Index(ctx, "test/repo", repo)

	ret := NewRetriever(db, nil)

	results, err := ret.SearchSymbol("test/repo", "capitalize", 5)
	if err != nil {
		t.Fatalf("SearchSymbol: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected to find 'capitalize' function")
	}
	if results[0].SymbolName != "capitalize" {
		t.Errorf("expected 'capitalize', got %q", results[0].SymbolName)
	}
}

func TestRetriever_EmptyRepo(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	ret := NewRetriever(db, nil)

	results, err := ret.Search(ctx, "nonexistent/repo", "anything", 10)
	if err != nil {
		t.Fatalf("Search on empty repo: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent repo, got %d", len(results))
	}
}

func TestMergeRRF(t *testing.T) {
	listA := []scoredChunk{
		{chunk: ChunkMatch{ID: 1, SymbolName: "foo"}, score: 10},
		{chunk: ChunkMatch{ID: 2, SymbolName: "bar"}, score: 8},
		{chunk: ChunkMatch{ID: 3, SymbolName: "baz"}, score: 6},
	}
	listB := []scoredChunk{
		{chunk: ChunkMatch{ID: 2, SymbolName: "bar"}, score: 0.9},
		{chunk: ChunkMatch{ID: 4, SymbolName: "qux"}, score: 0.8},
		{chunk: ChunkMatch{ID: 1, SymbolName: "foo"}, score: 0.7},
	}

	results := mergeRRF(listA, listB, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Items appearing in both lists should rank higher.
	// ID=1 (foo) is rank 0 in A, rank 2 in B → RRF = 1/61 + 1/63
	// ID=2 (bar) is rank 1 in A, rank 0 in B → RRF = 1/62 + 1/61
	// bar should be ranked first or second (it appears first in list B).
	topIDs := make(map[int64]bool)
	for _, r := range results {
		topIDs[r.ID] = true
	}
	if !topIDs[1] || !topIDs[2] {
		t.Error("expected IDs 1 and 2 to be in top results (they appear in both lists)")
	}
}

func TestFormatCodeContext(t *testing.T) {
	matches := []ChunkMatch{
		{
			FilePath:   "server.go",
			ChunkType:  "method",
			SymbolName: "Server.Start",
			StartLine:  10,
			EndLine:    15,
			Content:    "func (s *Server) Start() error {\n\treturn nil\n}",
		},
		{
			FilePath:   "server.go",
			ChunkType:  "function",
			SymbolName: "NewServer",
			StartLine:  5,
			EndLine:    8,
			Content:    "func NewServer(addr string) *Server {\n\treturn &Server{addr: addr}\n}",
		},
	}

	ctx := FormatCodeContext(matches)
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if len(ctx) < 50 {
		t.Fatalf("context too short: %q", ctx)
	}
}

func TestFormatCodeContext_Empty(t *testing.T) {
	ctx := FormatCodeContext(nil)
	if ctx != "" {
		t.Fatalf("expected empty string for no matches, got %q", ctx)
	}
}
