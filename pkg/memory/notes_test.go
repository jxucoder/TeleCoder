package memory

import (
	"testing"
)

func TestNoteStore_SetAndGet(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	if err := ns.Set("org/api", "auth_system", "Uses JWT with Redis sessions", "user_stated"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	note, err := ns.Get("org/api", "auth_system")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if note == nil {
		t.Fatal("expected note, got nil")
	}
	if note.Value != "Uses JWT with Redis sessions" {
		t.Errorf("got value %q", note.Value)
	}
	if note.Source != "user_stated" {
		t.Errorf("got source %q", note.Source)
	}
}

func TestNoteStore_Upsert(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	ns.Set("org/api", "testing", "Uses testify", "inferred")
	ns.Set("org/api", "testing", "Table-driven tests with testify", "llm_extracted")

	note, _ := ns.Get("org/api", "testing")
	if note.Value != "Table-driven tests with testify" {
		t.Errorf("upsert should replace value, got %q", note.Value)
	}
	if note.Source != "llm_extracted" {
		t.Errorf("upsert should update source, got %q", note.Source)
	}
}

func TestNoteStore_Append(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	ns.Set("org/api", "known_issues", "Race condition in payments", "inferred")
	ns.Append("org/api", "known_issues", "Missing null check in user service", "llm_extracted")

	note, _ := ns.Get("org/api", "known_issues")
	if note == nil {
		t.Fatal("expected note after append")
	}
	if note.Value != "Race condition in payments\nMissing null check in user service" {
		t.Errorf("append should concatenate values, got %q", note.Value)
	}
}

func TestNoteStore_AppendNew(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	// Append to non-existing key should create.
	ns.Append("org/api", "new_key", "first value", "inferred")

	note, _ := ns.Get("org/api", "new_key")
	if note == nil {
		t.Fatal("expected note after append to new key")
	}
	if note.Value != "first value" {
		t.Errorf("expected 'first value', got %q", note.Value)
	}
}

func TestNoteStore_List(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	ns.Set("org/api", "auth", "JWT", "inferred")
	ns.Set("org/api", "database", "PostgreSQL", "inferred")
	ns.Set("org/api", "testing", "testify", "inferred")
	ns.Set("other/repo", "auth", "OAuth", "inferred") // different repo

	notes, err := ns.List("org/api")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes for org/api, got %d", len(notes))
	}

	// Should be ordered by key.
	if notes[0].Key != "auth" || notes[1].Key != "database" || notes[2].Key != "testing" {
		t.Errorf("notes not sorted by key: %v, %v, %v", notes[0].Key, notes[1].Key, notes[2].Key)
	}
}

func TestNoteStore_Search(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	ns.Set("org/api", "auth_system", "Uses JWT tokens validated in middleware", "inferred")
	ns.Set("org/api", "database", "PostgreSQL with connection pooling", "inferred")
	ns.Set("org/api", "caching", "Redis for session cache and rate limiting", "inferred")

	results, err := ns.Search("org/api", "JWT")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'JWT', got %d", len(results))
	}
	if results[0].Key != "auth_system" {
		t.Errorf("expected auth_system note, got %q", results[0].Key)
	}
}

func TestNoteStore_Delete(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	ns.Set("org/api", "temp", "temporary note", "inferred")
	ns.Delete("org/api", "temp")

	note, _ := ns.Get("org/api", "temp")
	if note != nil {
		t.Error("expected note to be deleted")
	}
}

func TestNoteStore_GetNonexistent(t *testing.T) {
	db := testDB(t)
	ns := NewNoteStore(db)

	note, err := ns.Get("org/api", "nonexistent")
	if err != nil {
		t.Fatalf("Get nonexistent: %v", err)
	}
	if note != nil {
		t.Error("expected nil for nonexistent note")
	}
}

func TestFormatNotesContext(t *testing.T) {
	notes := []Note{
		{Key: "auth_system", Value: "JWT with Redis sessions"},
		{Key: "testing", Value: "Table-driven tests, testify assertions"},
	}

	ctx := FormatNotesContext(notes)
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if len(ctx) < 20 {
		t.Fatalf("context too short: %q", ctx)
	}
}

func TestFormatNotesContext_Empty(t *testing.T) {
	ctx := FormatNotesContext(nil)
	if ctx != "" {
		t.Fatalf("expected empty string, got %q", ctx)
	}
}
