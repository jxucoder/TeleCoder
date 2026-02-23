package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// testDB creates a temporary SQLite database with the codebase memory schema.
func testDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")

	// Run the codebase memory schema.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS codebase_files (
			repo       TEXT NOT NULL,
			path       TEXT NOT NULL,
			language   TEXT NOT NULL DEFAULT '',
			hash       TEXT NOT NULL DEFAULT '',
			indexed_at DATETIME NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (repo, path)
		);

		CREATE TABLE IF NOT EXISTS codebase_chunks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			repo        TEXT NOT NULL,
			file_path   TEXT NOT NULL,
			chunk_type  TEXT NOT NULL DEFAULT '',
			symbol_name TEXT NOT NULL DEFAULT '',
			start_line  INTEGER NOT NULL DEFAULT 0,
			end_line    INTEGER NOT NULL DEFAULT 0,
			content     TEXT NOT NULL,
			embedding   BLOB
		);

		CREATE INDEX IF NOT EXISTS idx_chunks_repo
			ON codebase_chunks(repo);
		CREATE INDEX IF NOT EXISTS idx_chunks_file
			ON codebase_chunks(repo, file_path);
		CREATE INDEX IF NOT EXISTS idx_chunks_symbol
			ON codebase_chunks(repo, symbol_name);

		CREATE TABLE IF NOT EXISTS codebase_notes (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			repo       TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL,
			source     TEXT NOT NULL DEFAULT 'inferred',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_notes_repo
			ON codebase_notes(repo);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_notes_repo_key
			ON codebase_notes(repo, key);

		CREATE TABLE IF NOT EXISTS codebase_index_state (
			repo          TEXT PRIMARY KEY,
			last_commit   TEXT NOT NULL DEFAULT '',
			last_indexed  DATETIME NOT NULL DEFAULT (datetime('now')),
			total_files   INTEGER NOT NULL DEFAULT 0,
			total_chunks  INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	// Create FTS5 table separately.
	db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS codebase_chunks_fts
		USING fts5(content, symbol_name, file_path, content=codebase_chunks, content_rowid=id)
	`)

	t.Cleanup(func() { db.Close() })
	return db
}

// testRepo creates a temporary directory with some Go source files.
func testRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create a simple Go project.
	writeFile(t, dir, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`)

	writeFile(t, dir, "server.go", `package main

import "net/http"

type Server struct {
	addr string
}

func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}
`)

	writeFile(t, dir, "utils.go", `package main

import "strings"

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
`)

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestCodebaseIndex_Index(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	idx := NewCodebaseIndex(db, nil) // no embedder for this test
	if err := idx.Index(ctx, "test/repo", repo); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Verify files were indexed.
	stats, err := idx.GetStats("test/repo")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalFiles != 3 {
		t.Errorf("expected 3 files indexed, got %d", stats.TotalFiles)
	}
	if stats.TotalChunks == 0 {
		t.Error("expected at least some chunks")
	}

	// Verify specific chunks exist.
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM codebase_chunks WHERE repo = ? AND symbol_name = ?",
		"test/repo", "NewServer",
	).Scan(&count)
	if count == 0 {
		t.Error("expected to find NewServer chunk")
	}

	db.QueryRow(
		"SELECT COUNT(*) FROM codebase_chunks WHERE repo = ? AND symbol_name = ?",
		"test/repo", "Server.Start",
	).Scan(&count)
	if count == 0 {
		t.Error("expected to find Server.Start method chunk")
	}
}

func TestCodebaseIndex_ReindexClears(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	idx := NewCodebaseIndex(db, nil)
	idx.Index(ctx, "test/repo", repo)

	// Re-index should clear and recount.
	idx.Index(ctx, "test/repo", repo)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM codebase_files WHERE repo = ?", "test/repo").Scan(&count)
	if count != 3 {
		t.Errorf("after reindex expected 3 files, got %d", count)
	}
}

func TestCodebaseIndex_WithEmbedder(t *testing.T) {
	db := testDB(t)
	repo := testRepo(t)
	ctx := context.Background()

	idx := NewCodebaseIndex(db, &mockEmbedder{})
	if err := idx.Index(ctx, "test/repo", repo); err != nil {
		t.Fatalf("Index with embedder: %v", err)
	}

	// Verify embeddings were stored.
	var nonNullCount int
	db.QueryRow(
		"SELECT COUNT(*) FROM codebase_chunks WHERE repo = ? AND embedding IS NOT NULL",
		"test/repo",
	).Scan(&nonNullCount)

	var totalCount int
	db.QueryRow("SELECT COUNT(*) FROM codebase_chunks WHERE repo = ?", "test/repo").Scan(&totalCount)

	if nonNullCount == 0 {
		t.Error("expected some chunks to have embeddings")
	}
	if nonNullCount != totalCount {
		t.Errorf("expected all %d chunks to have embeddings, got %d", totalCount, nonNullCount)
	}
}
