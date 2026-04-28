package db

import (
"context"
"database/sql"
"fmt"
"os"
"path/filepath"
"strings"

_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
if path == "" {
return nil, fmt.Errorf("database path is required")
}

if err := ensureDirectory(path); err != nil {
return nil, err
}

database, err := sql.Open("sqlite", path)
if err != nil {
return nil, fmt.Errorf("open sqlite database: %w", err)
}

database.SetMaxOpenConns(1)
database.SetMaxIdleConns(1)

if _, err := database.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
database.Close()
return nil, fmt.Errorf("configure sqlite busy timeout: %w", err)
}
if _, err := database.Exec("PRAGMA journal_mode = WAL;"); err != nil {
database.Close()
return nil, fmt.Errorf("configure sqlite journal mode: %w", err)
}

return database, nil
}

func Init(ctx context.Context, database *sql.DB) error {
statements := []string{
`CREATE TABLE IF NOT EXISTS pastes (
id TEXT PRIMARY KEY,
content TEXT NOT NULL,
content_bytes INTEGER NOT NULL,
expire_mode TEXT NOT NULL,
expires_at DATETIME,
burn_after_read INTEGER NOT NULL DEFAULT 0,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
viewed_at DATETIME
);`,
`CREATE INDEX IF NOT EXISTS idx_pastes_expires_at ON pastes(expires_at);`,
`CREATE INDEX IF NOT EXISTS idx_pastes_created_at ON pastes(created_at);`,
`CREATE TABLE IF NOT EXISTS rate_limits (
bucket TEXT NOT NULL,
key TEXT NOT NULL,
count INTEGER NOT NULL,
reset_at DATETIME NOT NULL,
PRIMARY KEY (bucket, key)
);`,
}

for _, statement := range statements {
if _, err := database.ExecContext(ctx, statement); err != nil {
return fmt.Errorf("initialize sqlite schema: %w", err)
}
}

return nil
}

func ensureDirectory(path string) error {
if path == ":memory:" || strings.HasPrefix(path, "file:") {
return nil
}
dir := filepath.Dir(path)
if dir == "." || dir == "" {
return nil
}
if err := os.MkdirAll(dir, 0o755); err != nil {
return fmt.Errorf("create database directory: %w", err)
}
return nil
}
