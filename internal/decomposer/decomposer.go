// Package decomposer splits complex tasks into ordered sub-tasks using an LLM.
//
// For simple tasks the LLM returns a single sub-task (no overhead). For complex
// multi-file changes it returns 2-5 ordered steps, each of which is run through
// the plan -> sandbox -> review cycle independently on the same branch.
package decomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jxucoder/opentl/internal/orchestrator"
)

// SubTask is a single step in a decomposed task.
type SubTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Decompose uses the LLM to break a task into ordered sub-tasks.
// If the task is simple, returns a single SubTask wrapping the original prompt.
// The repoContext (from the indexer) is included so the LLM can make informed
// decisions about how to split the work.
func Decompose(ctx context.Context, llm orchestrator.LLMClient, prompt, repoContext string) ([]SubTask, error) {
	user := fmt.Sprintf("Task: %s", prompt)
	if repoContext != "" {
		user = fmt.Sprintf("## Codebase Context\n%s\n\nTask: %s", repoContext, prompt)
	}

	response, err := llm.Complete(ctx, decomposerSystemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("decomposing task: %w", err)
	}

	tasks, err := parseSubTasks(response)
	if err != nil {
		// If parsing fails, fall back to a single task with the original prompt.
		return []SubTask{{Title: "Complete task", Description: prompt}}, nil
	}

	if len(tasks) == 0 {
		return []SubTask{{Title: "Complete task", Description: prompt}}, nil
	}

	return tasks, nil
}

// parseSubTasks extracts the JSON array from the LLM response.
// The response may contain markdown fences or extra text around the JSON.
func parseSubTasks(response string) ([]SubTask, error) {
	// Try to find a JSON array in the response.
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	var tasks []SubTask
	if err := json.Unmarshal([]byte(jsonStr), &tasks); err != nil {
		return nil, fmt.Errorf("parsing sub-tasks JSON: %w", err)
	}

	return tasks, nil
}

// extractJSON finds the first JSON array in the text, handling optional
// markdown code fences.
func extractJSON(s string) string {
	// Strip markdown code fences if present.
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (with optional language tag).
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	// Find the first '[' and last ']' to extract the array.
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end < 0 || end <= start {
		return ""
	}

	return s[start : end+1]
}

const decomposerSystemPrompt = `You are a task decomposition engine for a coding agent.

Given a task description (and optionally codebase context), decide whether the
task should be executed as a single step or broken into multiple ordered steps.

Rules:
- For simple, focused tasks (e.g. "fix the typo in README", "add a unit test
  for function X"), return a SINGLE sub-task.
- For complex, multi-concern tasks (e.g. "add user authentication with login,
  signup, and password reset"), break into 2-5 ordered steps.
- Each step should be independently executable and testable.
- Steps are executed sequentially on the same git branch -- later steps can
  depend on earlier steps' changes.
- Keep step descriptions specific and actionable.

Return ONLY a JSON array (no other text) in this exact format:

[
  {"title": "Short title", "description": "Detailed description of what to do"},
  {"title": "Short title", "description": "Detailed description of what to do"}
]

For a simple task, return a single-element array.`
