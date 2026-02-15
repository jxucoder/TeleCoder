package orchestrator

import (
	"context"
	"strings"
	"testing"
)

type fakeLLM struct {
	response string
	err      error
}

func (f *fakeLLM) Complete(ctx context.Context, system, user string) (string, error) {
	return f.response, f.err
}

func TestPlan(t *testing.T) {
	o := New(&fakeLLM{response: "Plan output"})
	got, err := o.Plan(context.Background(), "owner/repo", "fix bug", "repo ctx")
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if got != "Plan output" {
		t.Fatalf("unexpected plan: %s", got)
	}
}

func TestReviewApproved(t *testing.T) {
	o := New(&fakeLLM{response: "APPROVED: looks good"})
	res, err := o.Review(context.Background(), "task", "plan", "diff")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !res.Approved {
		t.Fatal("expected approved review")
	}
}

func TestReviewRevisionNeeded(t *testing.T) {
	o := New(&fakeLLM{response: "REVISION NEEDED: add test"})
	res, err := o.Review(context.Background(), "task", "plan", "diff")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if res.Approved {
		t.Fatal("expected non-approved review")
	}
}

func TestEnrichAndRevisePrompt(t *testing.T) {
	o := New(&fakeLLM{})
	enriched := o.EnrichPrompt("task", "plan")
	if !strings.Contains(enriched, "## Plan") {
		t.Fatalf("missing plan section: %s", enriched)
	}
	revised := o.RevisePrompt("task", "plan", "feedback")
	if !strings.Contains(revised, "Revision Instructions") {
		t.Fatalf("missing revision instructions: %s", revised)
	}
}

func TestDecomposeMultipleTasks(t *testing.T) {
	o := New(&fakeLLM{response: `[{"title":"T1","description":"D1"},{"title":"T2","description":"D2"}]`})
	tasks, err := o.Decompose(context.Background(), "task", "")
	if err != nil {
		t.Fatalf("decompose error: %v", err)
	}
	if len(tasks) != 2 || tasks[0].Title != "T1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestDecomposeFallsBackOnBadJSON(t *testing.T) {
	o := New(&fakeLLM{response: "not json"})
	tasks, err := o.Decompose(context.Background(), "original prompt", "")
	if err != nil {
		t.Fatalf("decompose error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Description != "original prompt" {
		t.Fatalf("expected fallback task, got: %+v", tasks)
	}
}

func TestExtractJSON(t *testing.T) {
	raw := "```json\n[{\"title\":\"A\",\"description\":\"B\"}]\n```"
	got := extractJSON(raw)
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("failed to extract json array: %q", got)
	}
}
