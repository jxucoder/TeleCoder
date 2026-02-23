# Codebase Memory v2 — Security + Extensibility Rewrite

## Problem

TeleCoder agents currently have zero memory between sessions and no understanding
of the codebase they operate on. The v1 memory (in-memory session summaries) and
the initial codebase indexing work address the core retrieval problem but miss two
critical dimensions:

1. **Security** — Code memory stores sensitive IP. Without isolation, tenant
   boundaries, secrets filtering, and access control, it becomes a liability.
2. **Extensibility** — The ecosystem has dozens of tools (Mem0, code-memory,
   OpenMemory, MCP servers). Tight coupling to one approach limits adoption.

This plan redesigns memory as a **secure, pluggable subsystem** that can be
extended with external tools and safely used in multi-tenant deployments.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                     TeleCoder Engine                      │
│                                                          │
│  enrichPrompt(ctx, repo, prompt) ──► MemoryManager       │
│                                        │                 │
│                                  ┌─────┴─────┐          │
│                                  │ SecurityGate│          │
│                                  └─────┬─────┘          │
│                           ┌────────────┼────────────┐    │
│                           ▼            ▼            ▼    │
│                     CodeMemory    SessionMemory  Notes   │
│                     Provider      Provider      Provider │
│                           │            │            │    │
│                     ┌─────┴─────┐      │      ┌────┴──┐ │
│                     ▼           ▼      ▼      ▼       ▼ │
│                  Built-in    MCP     Mem0   Built-in  K/V│
│                  (SQLite)   Server  Plugin  (SQLite)     │
└──────────────────────────────────────────────────────────┘
```

---

## 1. Security Layer

### 1.1 Threat Model

| Threat | Impact | Mitigation |
|--------|--------|------------|
| **Cross-repo data leak** | Agent working on repo A retrieves code from repo B | Repo-scoped isolation at every query |
| **Secrets in indexed code** | API keys, tokens, passwords stored in chunks | Pre-index secrets scanner + chunk redaction |
| **Memory poisoning** | Malicious PR injects misleading notes/chunks | Source-tracking + trust levels on notes |
| **Prompt injection via memory** | Retrieved context manipulates agent behavior | Content sanitization + context length limits |
| **Unauthorized index access** | External API caller reads proprietary code | Auth middleware on memory HTTP endpoints |
| **Embedding exfiltration** | Embeddings can be inverted to reconstruct source | Embeddings stored encrypted at rest |

### 1.2 SecretScanner Interface

```go
// pkg/memory/security.go

// SecretScanner detects secrets in code before indexing.
type SecretScanner interface {
    // ScanChunk returns findings in a chunk. If any are HIGH severity,
    // the chunk should be redacted or excluded.
    ScanChunk(content string) []SecretFinding
}

type SecretFinding struct {
    Type     string // "api_key", "password", "token", "private_key"
    Severity string // "HIGH", "MEDIUM", "LOW"
    Line     int
    Match    string // the matched pattern (partially masked)
}

// BuiltinScanner uses regexp patterns for common secret types.
// Covers: AWS keys, GitHub tokens, JWT, private keys, passwords in URLs,
// generic high-entropy strings.
type BuiltinScanner struct {
    patterns []secretPattern
}
```

**Integration point:** The `CodebaseIndex.insertChunk()` method runs the scanner
before storage. HIGH-severity findings cause the chunk to be either:
- **Redacted**: secrets replaced with `[REDACTED:api_key]` placeholders
- **Excluded**: chunk skipped entirely (configurable per severity)

### 1.3 Repo-Scoped Isolation

Every memory operation takes a `repo` parameter. This is already true in v1 but
needs hardening:

```go
// SecurityGate wraps any MemoryProvider and enforces repo-scoped access.
type SecurityGate struct {
    inner    MemoryProvider
    policies []AccessPolicy
}

type AccessPolicy struct {
    // AllowCrossRepo permits queries that span multiple repos
    // (e.g., "how does auth work across our microservices").
    // Default: false.
    AllowCrossRepo bool

    // AllowedRepos restricts which repos a caller can access.
    // Empty = all repos the caller owns.
    AllowedRepos []string

    // MaxContextBytes caps the total bytes of code context injected
    // into a prompt, preventing prompt injection via large payloads.
    MaxContextBytes int
}
```

### 1.4 Content Sanitization

Before injecting retrieved context into a prompt:

1. **Strip control sequences** — Remove `###TELECODER_` markers, shell escapes
2. **Cap content length** — Default 8KB per chunk, 32KB total context
3. **Detect prompt injection patterns** — Flag chunks containing instruction-like
   text ("ignore previous instructions", "you are now", etc.)
4. **Source attribution** — Every injected chunk includes `[source: file:line]`
   so the agent (and user) can trace where context came from

### 1.5 Audit Log

```go
// MemoryAuditEvent is emitted on every memory read/write.
type MemoryAuditEvent struct {
    Timestamp time.Time
    Operation string // "index", "search", "note_set", "note_delete"
    Repo      string
    SessionID string
    Query     string // for search ops
    Results   int    // number of results returned
    Redacted  int    // number of secrets redacted
}
```

Published to the EventBus so channels can surface audit info and operators can
pipe to external logging (ELK, Datadog, etc.).

---

## 2. Extensibility Layer

### 2.1 MemoryProvider Interface

The core insight: **memory is not one thing**. Code retrieval, session history,
and knowledge notes are different concerns with different optimal backends. The
system should compose multiple providers, not mandate one.

```go
// pkg/memory/provider.go

// CodeMemoryProvider retrieves relevant code context for a prompt.
type CodeMemoryProvider interface {
    Name() string
    Search(ctx context.Context, repo, query string, topK int) ([]CodeContext, error)
    SearchSymbol(ctx context.Context, repo, symbol string, topK int) ([]CodeContext, error)
    Index(ctx context.Context, repo, repoPath string) error
    Close() error
}

// SessionMemoryProvider retrieves relevant past session context.
type SessionMemoryProvider interface {
    Name() string
    Store(ctx context.Context, session SessionSummary) error
    Query(ctx context.Context, repo, prompt string, topK int) ([]SessionContext, error)
    Close() error
}

// NoteProvider manages durable knowledge about repos.
type NoteProvider interface {
    Name() string
    Set(ctx context.Context, repo, key, value string) error
    Get(ctx context.Context, repo, key string) (string, error)
    List(ctx context.Context, repo string) ([]KnowledgeNote, error)
    Search(ctx context.Context, repo, query string) ([]KnowledgeNote, error)
    Close() error
}

// CodeContext is the common currency between code memory providers.
type CodeContext struct {
    FilePath   string
    Symbol     string
    StartLine  int
    EndLine    int
    Content    string
    Score      float64
    Source     string // which provider returned this
}
```

### 2.2 MemoryManager (Composition Root)

```go
// MemoryManager composes multiple providers and applies security policies.
type MemoryManager struct {
    gate          *SecurityGate
    codeProviders []CodeMemoryProvider    // searched in order, results merged
    sessionMem    SessionMemoryProvider   // single provider
    notes         NoteProvider            // single provider
    scanner       SecretScanner
    auditor       AuditEmitter
}

// EnrichPrompt is the single entry point called by the engine.
func (m *MemoryManager) EnrichPrompt(ctx context.Context, repo, prompt string) string
```

The manager:
1. Queries all registered `CodeMemoryProvider`s in parallel
2. Merges results with RRF (same as current implementation)
3. Queries `NoteProvider` for repo knowledge
4. Queries `SessionMemoryProvider` for relevant past sessions
5. Passes everything through `SecurityGate` (redaction, size caps)
6. Formats and returns enriched prompt

### 2.3 Embedder Registry

```go
// pkg/memory/embedder.go

// Embedder generates vector embeddings from text.
// Unchanged interface — but now with a registry pattern matching CodingAgent.
type Embedder interface {
    Name() string
    Dimensions() int
    Embed(ctx context.Context, text string) ([]float64, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// Built-in implementations:
// - OpenAIEmbedder (text-embedding-3-small, 1536d)
// - VoyageEmbedder (voyage-code-3, 1024d) — optimized for code
// - OllamaEmbedder (nomic-embed-text, 768d) — local, no API key
// - NullEmbedder — returns nil, gracefully degrades to keyword-only
```

### 2.4 Chunker Registry

```go
// pkg/memory/chunker.go additions

// Chunker parses source files into semantic chunks.
type Chunker interface {
    Name() string
    Languages() []string // file extensions this chunker handles
    Chunk(filePath string, source []byte) []Chunk
}

// Built-in chunkers (already implemented):
// - GoASTChunker (uses go/parser, go/ast)
// - RegexChunker (Python, JS/TS, Rust, Java, Ruby)
// - LineChunker (fallback)

// Extension point: tree-sitter chunker for richer multi-language support.
// Register via: memory.RegisterChunker(&TreeSitterChunker{})
```

### 2.5 External Tool Integrations

Each integration implements one of the provider interfaces:

#### a) MCP Server (Expose Memory)

Expose TeleCoder's memory index as an MCP server so external tools
(Claude Desktop, Copilot, Cursor) can query it.

```go
// pkg/memory/mcp/server.go

// MCPMemoryServer exposes CodeMemoryProvider as MCP tools.
// Tools:
//   - telecoder_search_code(repo, query, top_k) → CodeContext[]
//   - telecoder_search_symbol(repo, symbol) → CodeContext[]
//   - telecoder_get_notes(repo) → KnowledgeNote[]
//   - telecoder_set_note(repo, key, value) → void
type MCPMemoryServer struct {
    manager *MemoryManager
}
```

#### b) MCP Client (Consume External Memory)

Use external MCP servers (like `code-memory`) as a `CodeMemoryProvider`:

```go
// pkg/memory/mcp/client.go

// MCPCodeProvider wraps an external MCP server as a CodeMemoryProvider.
type MCPCodeProvider struct {
    serverURL string
    client    mcp.Client
}
```

#### c) Mem0 Integration

```go
// pkg/memory/integrations/mem0.go

// Mem0Provider wraps Mem0's API as a SessionMemoryProvider.
// Stores session summaries in Mem0 for cross-tool memory sharing.
type Mem0Provider struct {
    apiKey  string
    baseURL string
}
```

#### d) Vector Database Backends

```go
// pkg/memory/vectordb/

// VectorStore abstracts vector similarity search.
// Default: brute-force over SQLite BLOBs (current impl).
// Extensions: Qdrant, Chroma, Pinecone, pgvector.
type VectorStore interface {
    Upsert(ctx context.Context, id string, vector []float64, metadata map[string]string) error
    Search(ctx context.Context, vector []float64, topK int, filter map[string]string) ([]VectorMatch, error)
    Delete(ctx context.Context, ids []string) error
}
```

### 2.6 Builder Pattern Extensions

```go
// telecoder.go additions

func (b *Builder) WithCodeMemory(p memory.CodeMemoryProvider) *Builder
func (b *Builder) WithSessionMemory(p memory.SessionMemoryProvider) *Builder
func (b *Builder) WithNotes(p memory.NoteProvider) *Builder
func (b *Builder) WithEmbedder(e memory.Embedder) *Builder
func (b *Builder) WithSecretScanner(s memory.SecretScanner) *Builder
func (b *Builder) WithMemoryPolicy(p memory.AccessPolicy) *Builder
```

---

## 3. Implementation Plan

### Phase 1: Security Hardening (Priority: HIGH)

| # | Task | Files | Est. |
|---|------|-------|------|
| 1.1 | **SecretScanner** — regex-based scanner for common patterns (AWS, GH tokens, JWTs, private keys, passwords) | `pkg/memory/security.go`, `security_test.go` | M |
| 1.2 | **Chunk redaction** — integrate scanner into `CodebaseIndex.insertChunk()`, redact or skip HIGH-severity chunks | `pkg/memory/codebase.go` | S |
| 1.3 | **Content sanitization** — strip control sequences, cap context length, detect prompt injection patterns | `pkg/memory/sanitize.go`, `sanitize_test.go` | M |
| 1.4 | **SecurityGate** — repo-scoped access enforcement, cross-repo prevention | `pkg/memory/gate.go`, `gate_test.go` | M |
| 1.5 | **Audit events** — emit `MemoryAuditEvent` on every operation via EventBus | `pkg/memory/audit.go` | S |
| 1.6 | **.gitignore/skip patterns** — ensure `.env`, `credentials.*`, `*.pem`, etc. are never indexed | `pkg/memory/chunker.go` (extend `IsIndexable`) | S |

### Phase 2: Provider Interfaces (Priority: HIGH)

| # | Task | Files |
|---|------|-------|
| 2.1 | **Define interfaces** — `CodeMemoryProvider`, `SessionMemoryProvider`, `NoteProvider`, `VectorStore`, `Chunker` | `pkg/memory/provider.go` |
| 2.2 | **MemoryManager** — composition root that merges providers + applies security | `pkg/memory/manager.go`, `manager_test.go` |
| 2.3 | **Refactor built-in** — wrap current SQLite-based code/notes as provider implementations | `pkg/memory/builtin/code.go`, `builtin/notes.go`, `builtin/session.go` |
| 2.4 | **Embedder registry** — `RegisterEmbedder()`, `NullEmbedder`, batch support | `pkg/memory/embedder.go` |
| 2.5 | **Chunker registry** — `RegisterChunker()`, dispatch by file extension | `pkg/memory/chunker.go` |
| 2.6 | **Builder integration** — `WithCodeMemory()`, `WithEmbedder()`, etc. on `telecoder.Builder` | `telecoder.go` |

### Phase 3: Embedder Implementations (Priority: MEDIUM)

| # | Task | Files |
|---|------|-------|
| 3.1 | **OpenAI embedder** — `text-embedding-3-small`, batched | `pkg/memory/embedders/openai.go` |
| 3.2 | **Voyage embedder** — `voyage-code-3`, code-optimized | `pkg/memory/embedders/voyage.go` |
| 3.3 | **Ollama embedder** — local `nomic-embed-text`, zero-API-key | `pkg/memory/embedders/ollama.go` |
| 3.4 | **Auto-detect** — pick embedder based on available env vars | `pkg/memory/embedders/auto.go` |

### Phase 4: MCP Integration (Priority: MEDIUM)

| # | Task | Files |
|---|------|-------|
| 4.1 | **MCP server** — expose memory as MCP tools (search_code, search_symbol, get/set_notes) | `pkg/memory/mcp/server.go` |
| 4.2 | **MCP client** — consume external MCP servers as CodeMemoryProvider | `pkg/memory/mcp/client.go` |
| 4.3 | **Config** — `telecoder config` wizard for MCP server/client setup | `cmd/telecoder/config.go` |

### Phase 5: External Integrations (Priority: LOW)

| # | Task | Files |
|---|------|-------|
| 5.1 | **Mem0 provider** — session memory via Mem0 API | `pkg/memory/integrations/mem0.go` |
| 5.2 | **Qdrant vector store** — swap SQLite BLOB search for Qdrant | `pkg/memory/vectordb/qdrant.go` |
| 5.3 | **Tree-sitter chunker** — richer multi-language parsing | `pkg/memory/chunkers/treesitter.go` |
| 5.4 | **Knowledge extraction** — post-session LLM pass to extract durable notes | `pkg/memory/extract.go` |

### Phase 6: Webhook-Driven Indexing (Priority: LOW)

| # | Task | Files |
|---|------|-------|
| 6.1 | **Push webhook handler** — reindex on git push events | `internal/httpapi/httpapi.go` |
| 6.2 | **Scheduled reindexing** — cron-based full reindex via `pkg/scheduler` | `pkg/scheduler/scheduler.go` |
| 6.3 | **Index status API** — `GET /api/memory/{repo}/stats` | `internal/httpapi/httpapi.go` |

---

## 4. Package Layout (Final State)

```
pkg/memory/
├── provider.go          # Core interfaces (CodeMemoryProvider, etc.)
├── manager.go           # MemoryManager (composition root)
├── gate.go              # SecurityGate (access control)
├── security.go          # SecretScanner + BuiltinScanner
├── sanitize.go          # Content sanitization + prompt injection detection
├── audit.go             # Audit event emission
├── chunker.go           # Chunk type + ChunkFile + IsIndexable (existing)
├── codebase.go          # CodebaseIndex (existing, refactored to implement CodeMemoryProvider)
├── retriever.go         # Retriever (existing, used internally by built-in provider)
├── notes.go             # NoteStore (existing, refactored to implement NoteProvider)
├── memory.go            # Session memory (existing, refactored to implement SessionMemoryProvider)
│
├── embedders/
│   ├── openai.go
│   ├── voyage.go
│   ├── ollama.go
│   └── auto.go
│
├── vectordb/
│   ├── vectordb.go      # VectorStore interface
│   ├── sqlite.go        # SQLite BLOB brute-force (default)
│   └── qdrant.go        # Qdrant client
│
├── mcp/
│   ├── server.go        # Expose memory as MCP tools
│   └── client.go        # Consume external MCP servers
│
├── integrations/
│   └── mem0.go          # Mem0 session memory provider
│
└── chunkers/
    └── treesitter.go    # Tree-sitter based chunker
```

---

## 5. Config Additions

```bash
# Memory configuration (~/.telecoder/config.env)

# Enable/disable codebase memory (default: true)
TELECODER_MEMORY_ENABLED=true

# Embedder selection: "openai", "voyage", "ollama", "none" (default: "auto")
TELECODER_EMBEDDER=auto

# Ollama endpoint for local embeddings (default: http://localhost:11434)
TELECODER_OLLAMA_URL=http://localhost:11434

# Security
TELECODER_MEMORY_SECRET_SCAN=true          # scan chunks for secrets
TELECODER_MEMORY_SECRET_ACTION=redact      # "redact" or "exclude"
TELECODER_MEMORY_CROSS_REPO=false          # allow cross-repo queries
TELECODER_MEMORY_MAX_CONTEXT_KB=32         # max context injected (KB)

# MCP
TELECODER_MCP_SERVER_ENABLED=false         # expose memory as MCP server
TELECODER_MCP_SERVER_PORT=7090
TELECODER_MCP_EXTERNAL_SERVERS=             # comma-separated MCP server URLs

# External integrations
TELECODER_MEM0_API_KEY=                    # Mem0 for session memory
TELECODER_QDRANT_URL=                      # Qdrant for vector search
```

---

## 6. Key Design Decisions

### Why interface-per-concern, not one big MemoryStore?

Code retrieval, session history, and knowledge notes have fundamentally different
access patterns, lifecycle, and optimal backends:

- **Code memory**: large, read-heavy, file-scoped, benefits from FTS + vectors
- **Session memory**: append-only, query-by-similarity, benefits from Mem0/Redis
- **Notes**: key-value, human-editable, rarely changes, fits SQLite perfectly

One interface would force backends to implement all three, discouraging adoption.

### Why SecurityGate as a wrapper, not embedded in providers?

Security should be **inescapable**. If each provider implements its own access
control, one buggy provider breaks the whole system. A single gate wrapping all
providers ensures consistent enforcement regardless of backend.

### Why MCP for both expose and consume?

MCP is the industry-standard protocol for tool/context integration (adopted by
Claude, Copilot, Cursor, etc.). By both exposing and consuming MCP:
- External tools can **query** TeleCoder's index (expose)
- TeleCoder can **leverage** external tools' indexes (consume)
- The ecosystem compounds instead of fragmenting

### Why start with SQLite, not a vector DB?

For repos under 100K lines (~5000 chunks), brute-force cosine similarity over
SQLite BLOBs is fast enough (<50ms). This avoids requiring Qdrant/Chroma in the
default deployment. The `VectorStore` interface allows upgrading to a dedicated
vector DB when scale demands it.

---

## 7. Security Checklist (Pre-Launch)

- [ ] Secrets scanner covers: AWS, GCP, Azure keys; GitHub/GitLab tokens;
      JWT secrets; private keys (RSA, EC, Ed25519); database URLs with passwords;
      generic high-entropy strings
- [ ] No `.env`, `.pem`, `.key`, `credentials.*` files are ever indexed
- [ ] Cross-repo queries are disabled by default
- [ ] Context injection is capped at configurable max bytes
- [ ] Prompt injection patterns are flagged in audit log
- [ ] Memory HTTP endpoints require authentication
- [ ] Embeddings are not exposed via any external API
- [ ] Audit events are emitted for every memory operation
- [ ] Notes have source-tracking (user_stated vs llm_extracted vs inferred)
- [ ] SecurityGate tests verify isolation with adversarial inputs
