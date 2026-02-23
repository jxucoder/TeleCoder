// Package sqlite implements store.SessionStore using SQLite.
package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jxucoder/TeleCoder/pkg/model"
)

// Store manages session and event persistence in SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at the given path.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id           TEXT PRIMARY KEY,
			repo         TEXT NOT NULL,
			prompt       TEXT NOT NULL,
			mode         TEXT NOT NULL DEFAULT 'task',
			status       TEXT NOT NULL DEFAULT 'pending',
			branch       TEXT NOT NULL DEFAULT '',
			agent        TEXT NOT NULL DEFAULT '',
			pr_url       TEXT NOT NULL DEFAULT '',
			pr_number    INTEGER NOT NULL DEFAULT 0,
			container_id TEXT NOT NULL DEFAULT '',
			error        TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS session_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			type       TEXT NOT NULL,
			data       TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);

		CREATE INDEX IF NOT EXISTS idx_events_session_id
			ON session_events(session_id);

		CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);

		CREATE INDEX IF NOT EXISTS idx_messages_session_id
			ON messages(session_id);
	`)
	if err != nil {
		return err
	}

	// Add agent column to existing databases (idempotent).
	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN agent TEXT NOT NULL DEFAULT ''`)

	// Add result columns to existing databases (idempotent).
	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN result_type TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN result_content TEXT NOT NULL DEFAULT ''`)

	if err := migrateCodebaseMemory(db); err != nil {
		return err
	}

	return nil
}

func migrateCodebaseMemory(db *sql.DB) error {
	_, err := db.Exec(`
		-- File-level metadata for indexed repos.
		CREATE TABLE IF NOT EXISTS codebase_files (
			repo       TEXT NOT NULL,
			path       TEXT NOT NULL,
			language   TEXT NOT NULL DEFAULT '',
			hash       TEXT NOT NULL DEFAULT '',
			indexed_at DATETIME NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (repo, path)
		);

		-- Code chunks: one per function/class/method/block.
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

		-- Knowledge notes: architecture decisions, conventions, patterns.
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

		-- Track last indexed commit per repo for incremental updates.
		CREATE TABLE IF NOT EXISTS codebase_index_state (
			repo          TEXT PRIMARY KEY,
			last_commit   TEXT NOT NULL DEFAULT '',
			last_indexed  DATETIME NOT NULL DEFAULT (datetime('now')),
			total_files   INTEGER NOT NULL DEFAULT 0,
			total_chunks  INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("codebase memory migration: %w", err)
	}

	// FTS5 virtual table for full-text search over chunks.
	// Separate exec because CREATE VIRTUAL TABLE can't be batched.
	_, _ = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS codebase_chunks_fts
		USING fts5(content, symbol_name, file_path, content=codebase_chunks, content_rowid=id)
	`)

	return nil
}

// DB returns the underlying database connection for use by other packages
// (e.g., codebase memory).
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateSession inserts a new session.
func (s *Store) CreateSession(sess *model.Session) error {
	if sess.Mode == "" {
		sess.Mode = model.ModeTask
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, repo, prompt, mode, status, branch, agent, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Repo, sess.Prompt, sess.Mode, sess.Status, sess.Branch,
		sess.Agent, sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(id string) (*model.Session, error) {
	row := s.db.QueryRow(
		`SELECT id, repo, prompt, mode, status, branch, agent, pr_url, pr_number,
		        container_id, error, result_type, result_content, created_at, updated_at
		 FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

// ListSessions returns all sessions ordered by creation time (newest first).
func (s *Store) ListSessions() ([]*model.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, repo, prompt, mode, status, branch, agent, pr_url, pr_number,
		        container_id, error, result_type, result_content, created_at, updated_at
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*model.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// UpdateSession updates mutable fields of a session.
func (s *Store) UpdateSession(sess *model.Session) error {
	sess.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE sessions SET
			status = ?, branch = ?, agent = ?, pr_url = ?, pr_number = ?,
			container_id = ?, error = ?, result_type = ?, result_content = ?,
			updated_at = ?
		 WHERE id = ?`,
		sess.Status, sess.Branch, sess.Agent, sess.PRUrl, sess.PRNumber,
		sess.ContainerID, sess.Error, string(sess.Result.Type), sess.Result.Content,
		sess.UpdatedAt, sess.ID,
	)
	return err
}

// AddEvent inserts a new event and returns its ID.
func (s *Store) AddEvent(event *model.Event) error {
	result, err := s.db.Exec(
		`INSERT INTO session_events (session_id, type, data, created_at)
		 VALUES (?, ?, ?, ?)`,
		event.SessionID, event.Type, event.Data, event.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	event.ID = id
	return nil
}

// GetEvents returns events for a session, optionally after a given event ID.
func (s *Store) GetEvents(sessionID string, afterID int64) ([]*model.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, type, data, created_at
		 FROM session_events
		 WHERE session_id = ? AND id > ?
		 ORDER BY id ASC`,
		sessionID, afterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		e := &model.Event{}
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetSessionByPR retrieves a session by its PR number and repository.
func (s *Store) GetSessionByPR(repo string, prNumber int) (*model.Session, error) {
	row := s.db.QueryRow(
		`SELECT id, repo, prompt, mode, status, branch, agent, pr_url, pr_number,
		        container_id, error, result_type, result_content, created_at, updated_at
		 FROM sessions
		 WHERE repo = ? AND pr_number = ?
		 ORDER BY created_at DESC
		 LIMIT 1`, repo, prNumber,
	)
	return scanSession(row)
}

// --- Message persistence for chat sessions ---

// AddMessage inserts a new message into a chat session.
func (s *Store) AddMessage(msg *model.Message) error {
	result, err := s.db.Exec(
		`INSERT INTO messages (session_id, role, content, created_at)
		 VALUES (?, ?, ?, ?)`,
		msg.SessionID, msg.Role, msg.Content, msg.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}

// GetMessages returns all messages for a session ordered by creation time.
func (s *Store) GetMessages(sessionID string) ([]*model.Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, created_at
		 FROM messages
		 WHERE session_id = ?
		 ORDER BY id ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*model.Message
	for rows.Next() {
		m := &model.Message{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// --- Scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanSession(row scannable) (*model.Session, error) {
	sess := &model.Session{}
	var resultType, resultContent string
	err := row.Scan(
		&sess.ID, &sess.Repo, &sess.Prompt, &sess.Mode, &sess.Status,
		&sess.Branch, &sess.Agent, &sess.PRUrl, &sess.PRNumber,
		&sess.ContainerID, &sess.Error, &resultType, &resultContent,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	sess.Result.Type = model.ResultType(resultType)
	sess.Result.Content = resultContent
	// Backfill Result PR fields from legacy top-level fields for consistency.
	if sess.PRUrl != "" {
		sess.Result.PRUrl = sess.PRUrl
		sess.Result.PRNumber = sess.PRNumber
		if sess.Result.Type == model.ResultNone {
			sess.Result.Type = model.ResultPR
		}
	}
	return sess, nil
}
