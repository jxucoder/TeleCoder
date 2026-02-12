// Package indexer provides repo-aware context indexing for the orchestrator.
//
// Before the planner generates a plan, the indexer fetches the repository
// structure (file tree, key config files, language breakdown) via the GitHub
// API so the LLM has real codebase context instead of just a repo name.
package indexer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	gogh "github.com/google/go-github/v68/github"
)

// keyFileNames are config / entry-point files whose content is included
// (first ~100 lines) so the planner understands the project setup.
var keyFileNames = map[string]bool{
	"README.md":       true,
	"package.json":    true,
	"go.mod":          true,
	"pyproject.toml":  true,
	"Cargo.toml":      true,
	"Makefile":        true,
	"Dockerfile":      true,
	"docker-compose.yml": true,
	"compose.yml":     true,
	"requirements.txt": true,
	"tsconfig.json":   true,
}

// maxTreeDepth limits the indented tree to the top N directory levels.
const maxTreeDepth = 3

// maxKeyFileLines caps how many lines of each key file are included.
const maxKeyFileLines = 100

// RepoContext holds the structural summary of a repository.
type RepoContext struct {
	Description string            // repo description from GitHub
	Tree        string            // indented file/directory listing
	Languages   map[string]int    // language name -> percentage
	KeyFiles    map[string]string // filename -> content snippet
}

// String formats the context as a single block of text suitable for injection
// into an LLM prompt.
func (rc *RepoContext) String() string {
	var b strings.Builder

	if rc.Description != "" {
		fmt.Fprintf(&b, "### Description\n%s\n\n", rc.Description)
	}

	if len(rc.Languages) > 0 {
		fmt.Fprintf(&b, "### Languages\n")
		// Sort by percentage descending.
		type langPct struct {
			name string
			pct  int
		}
		var langs []langPct
		for name, pct := range rc.Languages {
			langs = append(langs, langPct{name, pct})
		}
		sort.Slice(langs, func(i, j int) bool { return langs[i].pct > langs[j].pct })
		for _, l := range langs {
			fmt.Fprintf(&b, "- %s: %d%%\n", l.name, l.pct)
		}
		b.WriteString("\n")
	}

	if rc.Tree != "" {
		fmt.Fprintf(&b, "### File Tree (top %d levels)\n```\n%s\n```\n\n", maxTreeDepth, rc.Tree)
	}

	if len(rc.KeyFiles) > 0 {
		fmt.Fprintf(&b, "### Key Files\n")
		for name, content := range rc.KeyFiles {
			fmt.Fprintf(&b, "\n**%s**\n```\n%s\n```\n", name, content)
		}
	}

	return b.String()
}

// Index fetches repository metadata, file tree, and key files from the GitHub
// API and returns a structured RepoContext. The repo parameter should be in
// "owner/repo" format.
func Index(ctx context.Context, gh *gogh.Client, repo string) (*RepoContext, error) {
	owner, repoName, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	rc := &RepoContext{
		Languages: make(map[string]int),
		KeyFiles:  make(map[string]string),
	}

	// 1. Get repo metadata (description, default branch).
	repoInfo, _, err := gh.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("fetching repo info: %w", err)
	}
	rc.Description = repoInfo.GetDescription()
	defaultBranch := repoInfo.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// 2. Get languages.
	languages, _, err := gh.Repositories.ListLanguages(ctx, owner, repoName)
	if err == nil && len(languages) > 0 {
		var total int
		for _, bytes := range languages {
			total += bytes
		}
		if total > 0 {
			for lang, bytes := range languages {
				rc.Languages[lang] = (bytes * 100) / total
			}
		}
	}

	// 3. Get the recursive file tree.
	tree, _, err := gh.Git.GetTree(ctx, owner, repoName, defaultBranch, true)
	if err != nil {
		return nil, fmt.Errorf("fetching file tree: %w", err)
	}

	rc.Tree = buildTreeString(tree.Entries)

	// 4. Fetch key files content.
	for _, entry := range tree.Entries {
		path := entry.GetPath()
		baseName := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			baseName = path[idx+1:]
		}
		// Only fetch top-level key files (no slash in path).
		if !strings.Contains(path, "/") && keyFileNames[baseName] {
			content, err := fetchFileContent(ctx, gh, owner, repoName, path, defaultBranch)
			if err == nil && content != "" {
				rc.KeyFiles[path] = content
			}
		}
	}

	return rc, nil
}

// buildTreeString formats tree entries as an indented file listing,
// limited to maxTreeDepth levels.
func buildTreeString(entries []*gogh.TreeEntry) string {
	var lines []string
	for _, e := range entries {
		path := e.GetPath()
		depth := strings.Count(path, "/")
		if depth >= maxTreeDepth {
			continue
		}

		indent := strings.Repeat("  ", depth)
		name := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			name = path[idx+1:]
		}

		if e.GetType() == "tree" {
			lines = append(lines, fmt.Sprintf("%s%s/", indent, name))
		} else {
			lines = append(lines, fmt.Sprintf("%s%s", indent, name))
		}
	}
	return strings.Join(lines, "\n")
}

// fetchFileContent retrieves a file's content from GitHub, truncated to
// maxKeyFileLines. Returns the content as a string.
func fetchFileContent(ctx context.Context, gh *gogh.Client, owner, repo, path, ref string) (string, error) {
	opts := &gogh.RepositoryContentGetOptions{Ref: ref}
	file, _, _, err := gh.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", err
	}
	if file == nil {
		return "", nil
	}

	// GetContent handles base64 decoding internally.
	content, err := file.GetContent()
	if err != nil {
		return "", fmt.Errorf("decoding content for %s: %w", path, err)
	}

	return truncateLines(content, maxKeyFileLines), nil
}

// truncateLines keeps only the first n lines of s.
func truncateLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
		lines = append(lines, "... (truncated)")
	}
	return strings.Join(lines, "\n")
}

// splitRepo parses "owner/repo" into its components.
func splitRepo(fullName string) (string, string, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q, expected \"owner/repo\"", fullName)
	}
	return parts[0], parts[1], nil
}
