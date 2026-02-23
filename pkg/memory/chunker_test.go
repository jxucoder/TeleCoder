package memory

import (
	"strings"
	"testing"
)

func TestChunkGo_Functions(t *testing.T) {
	src := []byte(`package main

import "fmt"

func hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func goodbye(name string) string {
	return fmt.Sprintf("Goodbye, %s!", name)
}
`)

	chunks := ChunkFile("main.go", src)

	// Should have preamble + 2 functions.
	funcChunks := filterChunks(chunks, "function")
	if len(funcChunks) != 2 {
		t.Fatalf("expected 2 function chunks, got %d", len(funcChunks))
	}

	if funcChunks[0].SymbolName != "hello" {
		t.Errorf("expected first function 'hello', got %q", funcChunks[0].SymbolName)
	}
	if funcChunks[1].SymbolName != "goodbye" {
		t.Errorf("expected second function 'goodbye', got %q", funcChunks[1].SymbolName)
	}
}

func TestChunkGo_Methods(t *testing.T) {
	src := []byte(`package server

type Engine struct {
	name string
}

func (e *Engine) Start() error {
	return nil
}

func (e *Engine) Stop() {
}
`)

	chunks := ChunkFile("engine.go", src)

	methods := filterChunks(chunks, "method")
	if len(methods) != 2 {
		t.Fatalf("expected 2 method chunks, got %d", len(methods))
	}

	if methods[0].SymbolName != "Engine.Start" {
		t.Errorf("expected 'Engine.Start', got %q", methods[0].SymbolName)
	}
	if methods[1].SymbolName != "Engine.Stop" {
		t.Errorf("expected 'Engine.Stop', got %q", methods[1].SymbolName)
	}
}

func TestChunkGo_StructsAndInterfaces(t *testing.T) {
	src := []byte(`package store

type SessionStore interface {
	GetSession(id string) (*Session, error)
	CreateSession(sess *Session) error
}

type Config struct {
	Host string
	Port int
}
`)

	chunks := ChunkFile("store.go", src)

	ifaces := filterChunks(chunks, "interface")
	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface chunk, got %d", len(ifaces))
	}
	if ifaces[0].SymbolName != "SessionStore" {
		t.Errorf("expected 'SessionStore', got %q", ifaces[0].SymbolName)
	}

	structs := filterChunks(chunks, "struct")
	if len(structs) != 1 {
		t.Fatalf("expected 1 struct chunk, got %d", len(structs))
	}
	if structs[0].SymbolName != "Config" {
		t.Errorf("expected 'Config', got %q", structs[0].SymbolName)
	}
}

func TestChunkGo_Preamble(t *testing.T) {
	src := []byte(`package main

import (
	"fmt"
	"os"
)

var version = "1.0.0"

func main() {
	fmt.Println(version)
}
`)

	chunks := ChunkFile("main.go", src)
	preambles := filterChunks(chunks, "preamble")
	if len(preambles) == 0 {
		t.Fatal("expected a preamble chunk")
	}
	if !strings.Contains(preambles[0].Content, "import") {
		t.Error("preamble should contain import block")
	}
}

func TestChunkPython(t *testing.T) {
	src := []byte(`import os

class UserService:
    def __init__(self, db):
        self.db = db

    def get_user(self, user_id):
        return self.db.find(user_id)

def helper_function():
    return True
`)

	chunks := ChunkFile("service.py", src)
	if len(chunks) == 0 {
		t.Fatal("expected chunks from Python file")
	}

	classes := filterChunks(chunks, "class")
	if len(classes) == 0 {
		t.Fatal("expected at least one class chunk")
	}
	if classes[0].SymbolName != "UserService" {
		t.Errorf("expected 'UserService', got %q", classes[0].SymbolName)
	}
}

func TestChunkJavaScript(t *testing.T) {
	src := []byte(`import React from 'react';

export function App() {
  return <div>Hello</div>;
}

export class Component extends React.Component {
  render() {
    return null;
  }
}

const helper = () => {
  return true;
};
`)

	chunks := ChunkFile("app.jsx", src)
	if len(chunks) == 0 {
		t.Fatal("expected chunks from JSX file")
	}

	funcs := filterChunks(chunks, "function")
	if len(funcs) == 0 {
		t.Fatal("expected at least one function chunk")
	}
}

func TestChunkRust(t *testing.T) {
	src := []byte(`pub struct Engine {
    name: String,
}

impl Engine {
    pub fn new(name: &str) -> Self {
        Engine { name: name.to_string() }
    }

    pub fn start(&self) {
        println!("Starting {}", self.name);
    }
}

pub trait Runtime {
    fn execute(&self, cmd: &str) -> Result<String, Error>;
}
`)

	chunks := ChunkFile("engine.rs", src)
	if len(chunks) == 0 {
		t.Fatal("expected chunks from Rust file")
	}

	structs := filterChunks(chunks, "struct")
	if len(structs) == 0 {
		t.Fatal("expected at least one struct chunk")
	}
}

func TestChunkByLines_Fallback(t *testing.T) {
	// Unknown file extension falls back to line-based chunking.
	src := []byte("line1\nline2\nline3\nline4\nline5\n")
	chunks := ChunkFile("data.unknown", src)
	// Unknown extension has empty DetectLanguage, so IsIndexable returns false,
	// but ChunkFile itself still works.
	// Actually for unknown extensions, ChunkFile falls back to chunkByLines.
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk from fallback")
	}
	if chunks[0].ChunkType != "block" {
		t.Errorf("expected chunk type 'block', got %q", chunks[0].ChunkType)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.js", "javascript"},
		{"component.tsx", "tsx"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"app.rb", "ruby"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		got := DetectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsIndexable(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"pkg/store/sqlite.go", true},
		{"node_modules/foo/index.js", false},
		{"vendor/lib/lib.go", false},
		{".git/HEAD", false},
		{"bundle.min.js", false},
		{"go.sum", false},
		{"photo.png", false},
		{"data.csv", false}, // no language detected
	}

	for _, tt := range tests {
		got := IsIndexable(tt.path)
		if got != tt.want {
			t.Errorf("IsIndexable(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestEmbeddableText(t *testing.T) {
	chunk := Chunk{
		FilePath:   "pkg/server/handler.go",
		ChunkType:  "function",
		SymbolName: "handleRequest",
		Content:    "func handleRequest(w http.ResponseWriter, r *http.Request) {}",
	}

	text := EmbeddableText(chunk)
	if !strings.Contains(text, "# File: pkg/server/handler.go") {
		t.Error("embeddable text should contain file path")
	}
	if !strings.Contains(text, "# Function: handleRequest") {
		t.Error("embeddable text should contain symbol info")
	}
	if !strings.Contains(text, "func handleRequest") {
		t.Error("embeddable text should contain the code")
	}
}

// --- helpers ---

func filterChunks(chunks []Chunk, chunkType string) []Chunk {
	var result []Chunk
	for _, c := range chunks {
		if c.ChunkType == chunkType {
			result = append(result, c)
		}
	}
	return result
}
