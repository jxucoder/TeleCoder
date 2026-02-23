package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Note represents a piece of durable knowledge about a repo — architecture
// decisions, coding conventions, known issues, user preferences, etc.
type Note struct {
	ID        int64
	Repo      string
	Key       string // e.g. "auth_system", "testing_convention", "known_issues"
	Value     string
	Source    string // "user_stated", "llm_extracted", "inferred"
	UpdatedAt time.Time
}

// NoteStore provides CRUD operations for knowledge notes.
type NoteStore struct {
	db *sql.DB
}

// NewNoteStore creates a new NoteStore.
func NewNoteStore(db *sql.DB) *NoteStore {
	return &NoteStore{db: db}
}

// Set creates or updates a note. If a note with the same repo+key exists,
// it is replaced.
func (n *NoteStore) Set(repo, key, value, source string) error {
	if source == "" {
		source = "inferred"
	}
	_, err := n.db.Exec(
		`INSERT INTO codebase_notes (repo, key, value, source, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (repo, key) DO UPDATE SET
		   value = excluded.value,
		   source = excluded.source,
		   updated_at = excluded.updated_at`,
		repo, key, value, source, time.Now().UTC(),
	)
	return err
}

// Append appends to an existing note's value (useful for accumulating
// knowledge over multiple sessions).
func (n *NoteStore) Append(repo, key, value, source string) error {
	existing, err := n.Get(repo, key)
	if err != nil || existing == nil {
		return n.Set(repo, key, value, source)
	}
	combined := existing.Value + "\n" + value
	return n.Set(repo, key, combined, source)
}

// Get retrieves a specific note by repo and key.
func (n *NoteStore) Get(repo, key string) (*Note, error) {
	note := &Note{}
	err := n.db.QueryRow(
		`SELECT id, repo, key, value, source, updated_at
		 FROM codebase_notes
		 WHERE repo = ? AND key = ?`,
		repo, key,
	).Scan(&note.ID, &note.Repo, &note.Key, &note.Value, &note.Source, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return note, nil
}

// List returns all notes for a repo, ordered by key.
func (n *NoteStore) List(repo string) ([]Note, error) {
	rows, err := n.db.Query(
		`SELECT id, repo, key, value, source, updated_at
		 FROM codebase_notes
		 WHERE repo = ?
		 ORDER BY key`,
		repo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Repo, &note.Key, &note.Value, &note.Source, &note.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

// Search returns notes whose key or value contains the query string.
func (n *NoteStore) Search(repo, query string) ([]Note, error) {
	pattern := "%" + query + "%"
	rows, err := n.db.Query(
		`SELECT id, repo, key, value, source, updated_at
		 FROM codebase_notes
		 WHERE repo = ? AND (key LIKE ? OR value LIKE ?)
		 ORDER BY updated_at DESC`,
		repo, pattern, pattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Repo, &note.Key, &note.Value, &note.Source, &note.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

// Delete removes a note by repo and key.
func (n *NoteStore) Delete(repo, key string) error {
	_, err := n.db.Exec(
		"DELETE FROM codebase_notes WHERE repo = ? AND key = ?",
		repo, key,
	)
	return err
}

// FormatNotesContext builds a markdown string from notes suitable for
// injecting into an agent prompt.
func FormatNotesContext(notes []Note) string {
	if len(notes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Codebase Knowledge\n\n")
	for _, n := range notes {
		b.WriteString(fmt.Sprintf("**%s**: %s\n\n", n.Key, n.Value))
	}
	return b.String()
}
