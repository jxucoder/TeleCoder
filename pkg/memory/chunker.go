package memory

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Chunk represents a semantic unit of code extracted from a source file.
type Chunk struct {
	FilePath   string
	ChunkType  string // "function", "method", "class", "struct", "interface", "preamble"
	SymbolName string // e.g. "Engine.runSession", "handleWebhook"
	StartLine  int
	EndLine    int
	Content    string
}

// ChunkFile parses a source file and returns semantic chunks.
// For Go files it uses the stdlib AST parser; for other languages
// it falls back to regex-based extraction.
func ChunkFile(filePath string, source []byte) []Chunk {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return chunkGo(filePath, source)
	case ".py":
		return chunkByRegex(filePath, source, pythonPatterns)
	case ".js", ".jsx", ".ts", ".tsx":
		return chunkByRegex(filePath, source, jsPatterns)
	case ".rs":
		return chunkByRegex(filePath, source, rustPatterns)
	case ".java", ".kt":
		return chunkByRegex(filePath, source, javaPatterns)
	case ".rb":
		return chunkByRegex(filePath, source, rubyPatterns)
	default:
		return chunkByLines(filePath, source, 60)
	}
}

// --- Go chunker (using stdlib go/parser) ---

func chunkGo(filePath string, source []byte) []Chunk {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
	if err != nil {
		// Fall back to line-based chunking if parsing fails.
		return chunkByLines(filePath, source, 60)
	}

	lines := strings.Split(string(source), "\n")
	var chunks []Chunk

	// Collect top-level declarations.
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			chunk := goFuncChunk(fset, d, lines, filePath)
			chunks = append(chunks, chunk)

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					chunk := goTypeChunk(fset, d, s, lines, filePath)
					chunks = append(chunks, chunk)
				}
			}
		}
	}

	// If the file has a preamble (imports, package, top-level vars) that
	// isn't captured by declarations, extract it.
	if preamble := goPreamble(fset, f, lines, filePath); preamble.Content != "" {
		chunks = append([]Chunk{preamble}, chunks...)
	}

	if len(chunks) == 0 {
		return chunkByLines(filePath, source, 60)
	}
	return chunks
}

func goFuncChunk(fset *token.FileSet, fn *ast.FuncDecl, lines []string, filePath string) Chunk {
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line

	name := fn.Name.Name
	chunkType := "function"
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		chunkType = "method"
		recvType := exprName(fn.Recv.List[0].Type)
		if recvType != "" {
			name = recvType + "." + name
		}
	}

	content := extractLines(lines, start, end)
	return Chunk{
		FilePath:   filePath,
		ChunkType:  chunkType,
		SymbolName: name,
		StartLine:  start,
		EndLine:    end,
		Content:    content,
	}
}

func goTypeChunk(fset *token.FileSet, gen *ast.GenDecl, ts *ast.TypeSpec, lines []string, filePath string) Chunk {
	start := fset.Position(gen.Pos()).Line
	end := fset.Position(gen.End()).Line

	chunkType := "type"
	switch ts.Type.(type) {
	case *ast.StructType:
		chunkType = "struct"
	case *ast.InterfaceType:
		chunkType = "interface"
	}

	content := extractLines(lines, start, end)
	return Chunk{
		FilePath:   filePath,
		ChunkType:  chunkType,
		SymbolName: ts.Name.Name,
		StartLine:  start,
		EndLine:    end,
		Content:    content,
	}
}

func goPreamble(fset *token.FileSet, f *ast.File, lines []string, filePath string) Chunk {
	// Preamble = everything before the first declaration.
	firstDeclLine := len(lines)
	if len(f.Decls) > 0 {
		firstDeclLine = fset.Position(f.Decls[0].Pos()).Line - 1
	}

	if firstDeclLine <= 1 {
		return Chunk{}
	}

	content := extractLines(lines, 1, firstDeclLine)
	content = strings.TrimSpace(content)
	if content == "" {
		return Chunk{}
	}

	return Chunk{
		FilePath:   filePath,
		ChunkType:  "preamble",
		SymbolName: "",
		StartLine:  1,
		EndLine:    firstDeclLine,
		Content:    content,
	}
}

// exprName extracts the type name from a receiver expression (handles *T and T).
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return exprName(e.X)
	default:
		return ""
	}
}

// --- Regex-based chunker for non-Go languages ---

type langPattern struct {
	chunkType string
	pattern   *regexp.Regexp
}

var pythonPatterns = []langPattern{
	{"class", regexp.MustCompile(`^class\s+(\w+)`)},
	{"function", regexp.MustCompile(`^def\s+(\w+)`)},
	{"method", regexp.MustCompile(`^\s+def\s+(\w+)`)},
}

var jsPatterns = []langPattern{
	{"class", regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`)},
	{"function", regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)},
	{"function", regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`)},
	{"function", regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[a-zA-Z_]\w*)\s*=>`)},
}

var rustPatterns = []langPattern{
	{"struct", regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`)},
	{"function", regexp.MustCompile(`^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)},
	{"trait", regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`)},
	{"impl", regexp.MustCompile(`^impl(?:\s*<[^>]*>)?\s+(\w+)`)},
}

var javaPatterns = []langPattern{
	{"class", regexp.MustCompile(`^(?:public|private|protected)?\s*(?:abstract\s+)?class\s+(\w+)`)},
	{"interface", regexp.MustCompile(`^(?:public|private|protected)?\s*interface\s+(\w+)`)},
	{"method", regexp.MustCompile(`^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:\w+(?:<[^>]*>)?)\s+(\w+)\s*\(`)},
}

var rubyPatterns = []langPattern{
	{"class", regexp.MustCompile(`^class\s+(\w+)`)},
	{"module", regexp.MustCompile(`^module\s+(\w+)`)},
	{"method", regexp.MustCompile(`^\s*def\s+(\w+[?!]?)`)},
}

func chunkByRegex(filePath string, source []byte, patterns []langPattern) []Chunk {
	lines := strings.Split(string(source), "\n")
	var chunks []Chunk
	var current *regexAccum

	for i, line := range lines {
		lineNum := i + 1

		for _, p := range patterns {
			matches := p.pattern.FindStringSubmatch(line)
			if matches != nil {
				// Flush previous accumulator.
				if current != nil {
					chunks = append(chunks, current.toChunk(filePath, lines))
				}
				current = &regexAccum{
					chunkType:  p.chunkType,
					symbolName: matches[1],
					startLine:  lineNum,
					endLine:    lineNum,
				}
				break
			}
		}

		if current != nil {
			current.endLine = lineNum
		}
	}

	// Flush last accumulator.
	if current != nil {
		chunks = append(chunks, current.toChunk(filePath, lines))
	}

	if len(chunks) == 0 {
		return chunkByLines(filePath, source, 60)
	}
	return chunks
}

type regexAccum struct {
	chunkType  string
	symbolName string
	startLine  int
	endLine    int
}

func (r *regexAccum) toChunk(filePath string, lines []string) Chunk {
	return Chunk{
		FilePath:   filePath,
		ChunkType:  r.chunkType,
		SymbolName: r.symbolName,
		StartLine:  r.startLine,
		EndLine:    r.endLine,
		Content:    extractLines(lines, r.startLine, r.endLine),
	}
}

// --- Line-based fallback ---

func chunkByLines(filePath string, source []byte, maxLines int) []Chunk {
	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 {
		return nil
	}

	var chunks []Chunk
	for i := 0; i < len(lines); i += maxLines {
		end := i + maxLines
		if end > len(lines) {
			end = len(lines)
		}

		content := strings.Join(lines[i:end], "\n")
		content = strings.TrimRight(content, "\n ")
		if content == "" {
			continue
		}

		chunks = append(chunks, Chunk{
			FilePath:   filePath,
			ChunkType:  "block",
			SymbolName: fmt.Sprintf("lines_%d_%d", i+1, end),
			StartLine:  i + 1,
			EndLine:    end,
			Content:    content,
		})
	}
	return chunks
}

// --- Helpers ---

func extractLines(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// DetectLanguage returns a language identifier based on file extension.
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt":
		return "kotlin"
	case ".rb":
		return "ruby"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".sh", ".bash":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	default:
		return ""
	}
}

// IsIndexable returns true if the file should be indexed.
func IsIndexable(path string) bool {
	// Skip common non-source directories/files.
	skipPaths := []string{
		"node_modules/", "vendor/", ".git/", "__pycache__/",
		".venv/", "venv/", "dist/", "build/", ".next/",
		".cache/", "coverage/", ".nyc_output/",
	}
	for _, skip := range skipPaths {
		if strings.Contains(path, skip) {
			return false
		}
	}

	// Skip binary/lock/generated files.
	skipSuffixes := []string{
		".min.js", ".min.css", ".map",
		".lock", ".sum",
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
		".woff", ".woff2", ".ttf", ".eot",
		".pdf", ".zip", ".tar", ".gz",
		".exe", ".dll", ".so", ".dylib",
		".o", ".a",
	}
	lower := strings.ToLower(path)
	for _, suffix := range skipSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}

	return DetectLanguage(path) != ""
}

// EmbeddableText creates a context-enriched string for embedding.
// Prepends file path and symbol info so the embedding model captures
// where this code lives.
func EmbeddableText(c Chunk) string {
	var b strings.Builder
	b.WriteString("# File: ")
	b.WriteString(c.FilePath)
	b.WriteByte('\n')
	if c.SymbolName != "" {
		b.WriteString("# ")
		b.WriteString(titleCase(c.ChunkType))
		b.WriteString(": ")
		b.WriteString(c.SymbolName)
		b.WriteByte('\n')
	}
	b.WriteString(c.Content)
	return b.String()
}
