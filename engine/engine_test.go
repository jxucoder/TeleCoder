package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jxucoder/TeleCoder/eventbus"
	"github.com/jxucoder/TeleCoder/gitprovider"
	"github.com/jxucoder/TeleCoder/model"
	"github.com/jxucoder/TeleCoder/pipeline"
	"github.com/jxucoder/TeleCoder/sandbox"
	sqliteStore "github.com/jxucoder/TeleCoder/store/sqlite"
)

// --- stubs ---

type stubLLM struct{}

func (s *stubLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return `[{"title":"Complete task","description":"do the thing"}]`, nil
}

type stubSandbox struct {
	startCalls int
}

func (s *stubSandbox) Start(_ context.Context, _ sandbox.StartOptions) (string, error) {
	s.startCalls++
	return "stub-container", nil
}
func (s *stubSandbox) Stop(_ context.Context, _ string) error                             { return nil }
func (s *stubSandbox) Wait(_ context.Context, _ string) (int, error)                      { return 0, nil }
func (s *stubSandbox) StreamLogs(_ context.Context, _ string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *stubSandbox) Exec(_ context.Context, _ string, _ []string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *stubSandbox) ExecCollect(_ context.Context, _ string, _ []string) (string, error) { return "", nil }
func (s *stubSandbox) CommitAndPush(_ context.Context, _, _, _ string) error               { return nil }
func (s *stubSandbox) EnsureNetwork(_ context.Context, _ string) error                     { return nil }
func (s *stubSandbox) IsRunning(_ context.Context, _ string) bool                          { return true }

type stubScanner struct{}

func (s *stubScanner) Scan() bool   { return false }
func (s *stubScanner) Text() string { return "" }
func (s *stubScanner) Err() error   { return nil }
func (s *stubScanner) Close() error { return nil }

type stubGitProvider struct {
	createPRCalls int
}

func (s *stubGitProvider) CreatePR(_ context.Context, _ gitprovider.PROptions) (string, int, error) {
	s.createPRCalls++
	return "https://github.com/test/repo/pull/1", 1, nil
}
func (s *stubGitProvider) GetDefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}
func (s *stubGitProvider) IndexRepo(_ context.Context, _ string) (*gitprovider.RepoContext, error) {
	return &gitprovider.RepoContext{Tree: "README.md"}, nil
}
func (s *stubGitProvider) ReplyToPRComment(_ context.Context, _ string, _ int, _ string) error {
	return nil
}

// --- helpers ---

func testEngine(t *testing.T) (*Engine, *stubSandbox, *stubGitProvider) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := sqliteStore.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	bus := eventbus.NewInMemoryBus()
	sb := &stubSandbox{}
	git := &stubGitProvider{}
	llmClient := &stubLLM{}
	plan := pipeline.NewPlanStage(llmClient, "")
	review := pipeline.NewReviewStage(llmClient, "")
	decompose := pipeline.NewDecomposeStage(llmClient, "")
	verify := pipeline.NewVerifyStage(llmClient, "")

	eng := New(
		Config{
			DockerImage:     "test-image",
			MaxRevisions:    1,
			ChatIdleTimeout: 30 * time.Minute,
			ChatMaxMessages: 50,
		},
		st, bus, sb, git, plan, review, decompose, verify,
	)
	return eng, sb, git
}

// --- tests ---

func TestCreateAndRunSession(t *testing.T) {
	eng, sb, _ := testEngine(t)

	sess, err := eng.CreateAndRunSession("owner/repo", "fix the bug")
	if err != nil {
		t.Fatalf("CreateAndRunSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Repo != "owner/repo" {
		t.Fatalf("expected repo 'owner/repo', got %q", sess.Repo)
	}
	if sess.Mode != model.ModeTask {
		t.Fatalf("expected mode 'task', got %q", sess.Mode)
	}
	if sess.Status != model.StatusPending {
		t.Fatalf("expected status 'pending', got %q", sess.Status)
	}
	if !strings.HasPrefix(sess.Branch, "telecoder/") {
		t.Fatalf("expected branch prefix 'telecoder/', got %q", sess.Branch)
	}

	// Wait for the background goroutine to start the sandbox.
	time.Sleep(200 * time.Millisecond)

	if sb.startCalls < 1 {
		t.Fatalf("expected sandbox Start to be called, got %d calls", sb.startCalls)
	}
}

func TestCreateChatSession(t *testing.T) {
	eng, sb, _ := testEngine(t)

	sess, err := eng.CreateChatSession("owner/repo")
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	if sess.Mode != model.ModeChat {
		t.Fatalf("expected mode 'chat', got %q", sess.Mode)
	}
	if sess.Status != model.StatusPending {
		t.Fatalf("expected status 'pending', got %q", sess.Status)
	}

	// Wait for the background init to run.
	time.Sleep(200 * time.Millisecond)

	if sb.startCalls < 1 {
		t.Fatalf("expected sandbox Start to be called for chat init")
	}
}

func TestSessionStoredInDB(t *testing.T) {
	eng, _, _ := testEngine(t)

	sess, err := eng.CreateAndRunSession("owner/repo", "add tests")
	if err != nil {
		t.Fatalf("CreateAndRunSession: %v", err)
	}

	got, err := eng.Store().GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Prompt != "add tests" {
		t.Fatalf("expected prompt 'add tests', got %q", got.Prompt)
	}
}

func TestEmitEventStoredAndPublished(t *testing.T) {
	eng, _, _ := testEngine(t)

	sess, _ := eng.CreateAndRunSession("owner/repo", "task")
	ch := eng.Bus().Subscribe(sess.ID)
	defer eng.Bus().Unsubscribe(sess.ID, ch)

	// The background goroutine emits events. Wait a bit and check the store.
	time.Sleep(300 * time.Millisecond)

	events, err := eng.Store().GetEvents(sess.ID, 0)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event to be stored")
	}
}

func TestDispatchLogLine(t *testing.T) {
	eng, _, _ := testEngine(t)

	sess, _ := eng.CreateAndRunSession("owner/repo", "task")
	// Wait for session to be created.
	time.Sleep(100 * time.Millisecond)

	// Directly test dispatchLogLine by calling it.
	eng.dispatchLogLine(sess.ID, "###TELECODER_STATUS### Cloning repo")
	eng.dispatchLogLine(sess.ID, "###TELECODER_ERROR### something failed")
	eng.dispatchLogLine(sess.ID, "###TELECODER_DONE### telecoder/abc123")
	eng.dispatchLogLine(sess.ID, "regular log line")

	events, _ := eng.Store().GetEvents(sess.ID, 0)
	statusFound := false
	errorFound := false
	outputFound := false
	for _, e := range events {
		if e.Type == "status" && e.Data == "Cloning repo" {
			statusFound = true
		}
		if e.Type == "error" && e.Data == "something failed" {
			errorFound = true
		}
		if e.Type == "output" && e.Data == "regular log line" {
			outputFound = true
		}
	}
	if !statusFound {
		t.Fatal("expected STATUS dispatch event")
	}
	if !errorFound {
		t.Fatal("expected ERROR dispatch event")
	}
	if !outputFound {
		t.Fatal("expected OUTPUT dispatch event")
	}
}

func TestFailSession(t *testing.T) {
	eng, _, _ := testEngine(t)

	sess, _ := eng.CreateAndRunSession("owner/repo", "task")
	time.Sleep(200 * time.Millisecond)

	got, _ := eng.Store().GetSession(sess.ID)
	eng.failSession(got, "test error")

	updated, _ := eng.Store().GetSession(sess.ID)
	if updated.Status != model.StatusError {
		t.Fatalf("expected status 'error', got %q", updated.Status)
	}
	if updated.Error != "test error" {
		t.Fatalf("expected error 'test error', got %q", updated.Error)
	}
}

func TestEngineStartAndStop(t *testing.T) {
	eng, _, _ := testEngine(t)

	ctx, cancel := context.WithCancel(context.Background())
	eng.Start(ctx)

	// Engine should be running. Stop it.
	cancel()
	eng.Stop()

	// Should not panic or hang.
}

// --- Agent selection tests ---

func TestResolveAgentName_SessionOverride(t *testing.T) {
	eng, _, _ := testEngine(t)

	// Per-session agent should take priority.
	got := eng.resolveAgentName("claude-code")
	if got != "claude-code" {
		t.Fatalf("expected 'claude-code', got %q", got)
	}
}

func TestResolveAgentName_CodeAgentConfig(t *testing.T) {
	eng, _, _ := testEngine(t)
	eng.config.CodeAgent = &AgentConfig{Name: "codex"}

	// No session override → falls back to CodeAgent config.
	got := eng.resolveAgentName("")
	if got != "codex" {
		t.Fatalf("expected 'codex', got %q", got)
	}
}

func TestResolveAgentName_DefaultAgent(t *testing.T) {
	eng, _, _ := testEngine(t)
	eng.config.Agent = "opencode"

	got := eng.resolveAgentName("")
	if got != "opencode" {
		t.Fatalf("expected 'opencode', got %q", got)
	}
}

func TestResolveAgentName_AutoReturnsEmpty(t *testing.T) {
	eng, _, _ := testEngine(t)
	eng.config.Agent = "auto"

	got := eng.resolveAgentName("")
	if got != "" {
		t.Fatalf("expected empty string for auto, got %q", got)
	}

	// "auto" as session override should also return empty.
	got = eng.resolveAgentName("auto")
	if got != "" {
		t.Fatalf("expected empty string for session auto, got %q", got)
	}
}

func TestResolveAgentName_Priority(t *testing.T) {
	eng, _, _ := testEngine(t)
	eng.config.Agent = "opencode"
	eng.config.CodeAgent = &AgentConfig{Name: "codex"}

	// Session override > CodeAgent > Agent.
	got := eng.resolveAgentName("claude-code")
	if got != "claude-code" {
		t.Fatalf("expected session override 'claude-code', got %q", got)
	}

	// No session override: CodeAgent > Agent.
	got = eng.resolveAgentName("")
	if got != "codex" {
		t.Fatalf("expected CodeAgent 'codex', got %q", got)
	}

	// Remove CodeAgent: falls through to Agent.
	eng.config.CodeAgent = nil
	got = eng.resolveAgentName("")
	if got != "opencode" {
		t.Fatalf("expected default Agent 'opencode', got %q", got)
	}
}

func TestResolveAgentImage_Override(t *testing.T) {
	eng, _, _ := testEngine(t)

	// No agent config: uses default image.
	got := eng.resolveAgentImage(nil)
	if got != "test-image" {
		t.Fatalf("expected default 'test-image', got %q", got)
	}

	// Agent config without image: uses default.
	got = eng.resolveAgentImage(&AgentConfig{Name: "codex"})
	if got != "test-image" {
		t.Fatalf("expected default 'test-image', got %q", got)
	}

	// Agent config with image: uses override.
	got = eng.resolveAgentImage(&AgentConfig{Name: "codex", Image: "custom-image"})
	if got != "custom-image" {
		t.Fatalf("expected 'custom-image', got %q", got)
	}
}

func TestAgentEnv(t *testing.T) {
	base := []string{"GITHUB_TOKEN=abc", "ANTHROPIC_API_KEY=xyz"}

	// Nil agent config: returns copy of base.
	env := agentEnv(nil, base)
	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(env))
	}

	// Agent config with name and model.
	ac := &AgentConfig{Name: "claude-code", Model: "claude-sonnet-4-20250514"}
	env = agentEnv(ac, base)
	if len(env) != 4 {
		t.Fatalf("expected 4 env vars, got %d", len(env))
	}
	foundAgent := false
	foundModel := false
	for _, e := range env {
		if e == "TELECODER_AGENT=claude-code" {
			foundAgent = true
		}
		if e == "TELECODER_AGENT_MODEL=claude-sonnet-4-20250514" {
			foundModel = true
		}
	}
	if !foundAgent {
		t.Fatal("expected TELECODER_AGENT=claude-code in env")
	}
	if !foundModel {
		t.Fatal("expected TELECODER_AGENT_MODEL in env")
	}

	// Auto agent name should not be added.
	ac = &AgentConfig{Name: "auto"}
	env = agentEnv(ac, base)
	for _, e := range env {
		if strings.HasPrefix(e, "TELECODER_AGENT=") {
			t.Fatalf("auto agent should not add TELECODER_AGENT, got %q", e)
		}
	}
}

func TestAgentEnv_DoesNotMutateBase(t *testing.T) {
	base := []string{"GITHUB_TOKEN=abc"}
	ac := &AgentConfig{Name: "opencode", Model: "custom-model"}

	env := agentEnv(ac, base)

	// env should have more entries than base.
	if len(env) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(env))
	}
	// base should be unchanged.
	if len(base) != 1 {
		t.Fatalf("base was mutated: expected 1, got %d", len(base))
	}
}

func TestCreateAndRunSessionWithAgent(t *testing.T) {
	eng, sb, _ := testEngine(t)

	sess, err := eng.CreateAndRunSessionWithAgent("owner/repo", "fix the bug", "claude-code")
	if err != nil {
		t.Fatalf("CreateAndRunSessionWithAgent: %v", err)
	}
	if sess.Agent != "claude-code" {
		t.Fatalf("expected agent 'claude-code', got %q", sess.Agent)
	}

	// Verify session is persisted with the agent field.
	got, err := eng.Store().GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Agent != "claude-code" {
		t.Fatalf("expected persisted agent 'claude-code', got %q", got.Agent)
	}

	// Wait for the background goroutine to start the sandbox.
	time.Sleep(200 * time.Millisecond)

	if sb.startCalls < 1 {
		t.Fatalf("expected sandbox Start to be called, got %d calls", sb.startCalls)
	}
}

func TestRunAgentStage(t *testing.T) {
	eng, sb, _ := testEngine(t)

	sess := &model.Session{
		ID:     "test-agent-stage",
		Repo:   "owner/repo",
		Branch: "telecoder/test",
	}
	if err := eng.store.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	ac := AgentConfig{Name: "opencode", Image: "custom-research-image"}

	output, err := eng.runAgentStage(ctx, sess, ac, "explore the codebase")
	if err != nil {
		t.Fatalf("runAgentStage: %v", err)
	}

	// Stub returns empty output, but the call should succeed.
	_ = output

	// Sandbox Start should have been called (for the agent stage container).
	if sb.startCalls < 1 {
		t.Fatal("expected sandbox Start to be called for agent stage")
	}
}

func TestRunSandboxRoundWithAgent_PassesAgentEnv(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := sqliteStore.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	bus := eventbus.NewInMemoryBus()

	// Use a capturing sandbox that records start options.
	capSb := &capturingStubSandbox{}
	git := &stubGitProvider{}
	llmClient := &stubLLM{}

	eng := New(
		Config{
			DockerImage:     "test-image",
			MaxRevisions:    1,
			ChatIdleTimeout: 30 * time.Minute,
			ChatMaxMessages: 50,
			SandboxEnv:      []string{"GITHUB_TOKEN=abc"},
			CodeAgent:       &AgentConfig{Name: "claude-code", Model: "claude-sonnet-4-20250514"},
		},
		st, bus, capSb, git,
		pipeline.NewPlanStage(llmClient, ""),
		pipeline.NewReviewStage(llmClient, ""),
		pipeline.NewDecomposeStage(llmClient, ""),
		pipeline.NewVerifyStage(llmClient, ""),
	)

	sess := &model.Session{
		ID:     "test-env-pass",
		Repo:   "owner/repo",
		Branch: "telecoder/test",
		Status: model.StatusPending,
	}
	if err := st.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	_, err = eng.runSandboxRoundWithAgent(ctx, sess, "do the thing", "")
	if err != nil {
		t.Fatalf("runSandboxRoundWithAgent: %v", err)
	}

	// Verify the sandbox was started with the correct agent env.
	if capSb.lastOpts == nil {
		t.Fatal("expected sandbox Start to be called")
	}

	foundAgent := false
	foundModel := false
	for _, e := range capSb.lastOpts.Env {
		if e == "TELECODER_AGENT=claude-code" {
			foundAgent = true
		}
		if e == "TELECODER_AGENT_MODEL=claude-sonnet-4-20250514" {
			foundModel = true
		}
	}
	if !foundAgent {
		t.Fatalf("expected TELECODER_AGENT=claude-code in sandbox env, got %v", capSb.lastOpts.Env)
	}
	if !foundModel {
		t.Fatalf("expected TELECODER_AGENT_MODEL=claude-sonnet-4-20250514 in sandbox env, got %v", capSb.lastOpts.Env)
	}
}

func TestResearchAgentRunsBeforeDecompose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := sqliteStore.New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	bus := eventbus.NewInMemoryBus()
	recSb := &recordingStubSandbox{}
	git := &stubGitProvider{}
	llmClient := &stubLLM{}

	eng := New(
		Config{
			DockerImage:     "test-image",
			MaxRevisions:    0,
			ChatIdleTimeout: 30 * time.Minute,
			ChatMaxMessages: 50,
			ResearchAgent:   &AgentConfig{Name: "opencode", Image: "research-image"},
		},
		st, bus, recSb, git,
		pipeline.NewPlanStage(llmClient, ""),
		pipeline.NewReviewStage(llmClient, ""),
		pipeline.NewDecomposeStage(llmClient, ""),
		pipeline.NewVerifyStage(llmClient, ""),
	)

	sess, err := eng.CreateAndRunSession("owner/repo", "add rate limiting")
	if err != nil {
		t.Fatalf("CreateAndRunSession: %v", err)
	}

	// Wait for the background goroutine to complete.
	time.Sleep(500 * time.Millisecond)

	// Verify the sandbox was started at least twice:
	// once for the research agent, once for the coding stage.
	if recSb.startCount < 2 {
		t.Fatalf("expected at least 2 sandbox Start calls (research + code), got %d", recSb.startCount)
	}

	// The first Start call should use the research image.
	if len(recSb.allOpts) < 2 {
		t.Fatalf("expected at least 2 start options recorded, got %d", len(recSb.allOpts))
	}
	if recSb.allOpts[0].Image != "research-image" {
		t.Fatalf("expected first sandbox Start to use research image, got %q", recSb.allOpts[0].Image)
	}

	// The research prompt should mention "Explore this codebase".
	if !strings.Contains(recSb.allOpts[0].Prompt, "Explore this codebase") {
		t.Fatalf("expected research prompt, got %q", recSb.allOpts[0].Prompt)
	}

	_ = sess
}

func TestReviewAgentParseApproval(t *testing.T) {
	// Verify the APPROVED keyword parsing logic used in runSubTask.
	// This is inline in the engine, so we test the string matching directly.
	tests := []struct {
		feedback string
		approved bool
	}{
		{"APPROVED - looks good", true},
		{"The changes are approved and ready to merge", true},
		{"APPROVED", true},
		{"Needs changes: the error handling is missing", false},
		{"Changes look great. APPROVED.", true},
		{"Please fix the null pointer dereference on line 42", false},
	}

	for _, tt := range tests {
		got := strings.Contains(strings.ToUpper(tt.feedback), "APPROVED")
		if got != tt.approved {
			t.Errorf("feedback=%q: expected approved=%v, got %v", tt.feedback, tt.approved, got)
		}
	}
}

// --- Recording stubs ---

// capturingStubSandbox records the StartOptions from the last Start call.
type capturingStubSandbox struct {
	lastOpts *sandbox.StartOptions
}

func (s *capturingStubSandbox) Start(_ context.Context, opts sandbox.StartOptions) (string, error) {
	s.lastOpts = &opts
	return "cap-container", nil
}
func (s *capturingStubSandbox) Stop(_ context.Context, _ string) error                             { return nil }
func (s *capturingStubSandbox) Wait(_ context.Context, _ string) (int, error)                      { return 0, nil }
func (s *capturingStubSandbox) StreamLogs(_ context.Context, _ string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *capturingStubSandbox) Exec(_ context.Context, _ string, _ []string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *capturingStubSandbox) ExecCollect(_ context.Context, _ string, _ []string) (string, error) { return "", nil }
func (s *capturingStubSandbox) CommitAndPush(_ context.Context, _, _, _ string) error               { return nil }
func (s *capturingStubSandbox) EnsureNetwork(_ context.Context, _ string) error                     { return nil }
func (s *capturingStubSandbox) IsRunning(_ context.Context, _ string) bool                          { return true }

// recordingStubSandbox records all Start calls for multi-stage verification.
type recordingStubSandbox struct {
	startCount int
	allOpts    []sandbox.StartOptions
}

func (s *recordingStubSandbox) Start(_ context.Context, opts sandbox.StartOptions) (string, error) {
	s.startCount++
	s.allOpts = append(s.allOpts, opts)
	return fmt.Sprintf("rec-container-%d", s.startCount), nil
}
func (s *recordingStubSandbox) Stop(_ context.Context, _ string) error                             { return nil }
func (s *recordingStubSandbox) Wait(_ context.Context, _ string) (int, error)                      { return 0, nil }
func (s *recordingStubSandbox) StreamLogs(_ context.Context, _ string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *recordingStubSandbox) Exec(_ context.Context, _ string, _ []string) (sandbox.LineScanner, error) { return &stubScanner{}, nil }
func (s *recordingStubSandbox) ExecCollect(_ context.Context, _ string, _ []string) (string, error) { return "", nil }
func (s *recordingStubSandbox) CommitAndPush(_ context.Context, _, _, _ string) error               { return nil }
func (s *recordingStubSandbox) EnsureNetwork(_ context.Context, _ string) error                     { return nil }
func (s *recordingStubSandbox) IsRunning(_ context.Context, _ string) bool                          { return true }
