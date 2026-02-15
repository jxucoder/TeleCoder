package session

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestSessionCRUD(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	sess := &Session{
		ID:        "abc12345",
		Repo:      "owner/repo",
		Prompt:    "add tests",
		Mode:      ModeTask,
		Status:    StatusPending,
		Branch:    "opentl/abc12345",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ID != sess.ID || got.Repo != sess.Repo || got.Status != StatusPending {
		t.Fatalf("unexpected session: %+v", got)
	}

	got.Status = StatusRunning
	got.Error = "none"
	if err := store.UpdateSession(got); err != nil {
		t.Fatalf("update session: %v", err)
	}

	got2, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	if got2.Status != StatusRunning {
		t.Fatalf("status not updated: %s", got2.Status)
	}
}

func TestMessagesAndEvents(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()
	sess := &Session{
		ID:        "evt12345",
		Repo:      "owner/repo",
		Prompt:    "prompt",
		Mode:      ModeChat,
		Status:    StatusIdle,
		Branch:    "opentl/evt12345",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	msg := &Message{
		SessionID: sess.ID,
		Role:      "user",
		Content:   "hello",
		CreatedAt: now,
	}
	if err := store.AddMessage(msg); err != nil {
		t.Fatalf("add message: %v", err)
	}
	msgs, err := store.GetMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}

	ev := &Event{
		SessionID: sess.ID,
		Type:      "status",
		Data:      "Running",
		CreatedAt: now,
	}
	if err := store.AddEvent(ev); err != nil {
		t.Fatalf("add event: %v", err)
	}
	events, err := store.GetEvents(sess.ID, 0)
	if err != nil {
		t.Fatalf("get events: %v", err)
	}
	if len(events) != 1 || events[0].Data != "Running" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestEventBusSubscribePublishUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("s1")

	ev := &Event{SessionID: "s1", Type: "status", Data: "ok"}
	bus.Publish("s1", ev)

	select {
	case got := <-ch:
		if got.Data != "ok" {
			t.Fatalf("unexpected event data: %s", got.Data)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("did not receive event")
	}

	bus.Unsubscribe("s1", ch)
}

func TestEventBusDoesNotBlockOnSlowSubscriber(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("s2")

	// Fill channel to capacity (64) without reading.
	for i := 0; i < 64; i++ {
		bus.Publish("s2", &Event{SessionID: "s2", Type: "output", Data: "x"})
	}

	done := make(chan struct{})
	go func() {
		// This publish should be dropped and return immediately.
		bus.Publish("s2", &Event{SessionID: "s2", Type: "output", Data: "overflow"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("publish blocked on full channel")
	}

	bus.Unsubscribe("s2", ch)
}

func TestTruncateRuneSafe(t *testing.T) {
	got := Truncate("你好世界hello", 6)
	if got == "" {
		t.Fatal("expected non-empty truncate result")
	}
}
