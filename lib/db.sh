#!/usr/bin/env bash
# Database helpers — thin wrappers around sqlite3 CLI.

TC_DB="${TELECODER_DATA}/telecoder.db"

db_init() {
    sqlite3 "$TC_DB" <<'SQL'
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    repo_url    TEXT,
    repo_path   TEXT,
    workspace   TEXT NOT NULL,
    branch      TEXT,
    status      TEXT NOT NULL DEFAULT 'created',
    prompt      TEXT,
    test_cmd    TEXT,
    lint_cmd    TEXT,
    pid         INTEGER,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
SQL
}

db_query() {
    sqlite3 -separator '|' "$TC_DB" "$1"
}

db_exec() {
    sqlite3 "$TC_DB" "$1"
}
