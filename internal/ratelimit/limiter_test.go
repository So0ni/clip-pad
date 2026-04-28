package ratelimit

import (
"context"
"testing"
"time"

clipdb "github.com/So0ni/clip-pad/internal/db"
)

func TestLimiterPerMinutePerDayAndGlobal(t *testing.T) {
database := openLimiterTestDB(t)
limiter := New(Config{PerIPPerMinute: 2, PerIPPerDay: 3, GlobalPerDay: 4})
now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
ctx := context.Background()

for i := 0; i < 2; i++ {
tx, err := database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, "ip-a", now); err != nil {
t.Fatalf("CheckAndIncrementTx() error = %v", err)
}
if err := tx.Commit(); err != nil {
t.Fatalf("Commit() error = %v", err)
}
}

tx, err := database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, "ip-a", now); err != ErrRateLimitExceeded {
t.Fatalf("CheckAndIncrementTx() error = %v, want %v", err, ErrRateLimitExceeded)
}
_ = tx.Rollback()

nextMinute := now.Add(time.Minute)
for i := 0; i < 1; i++ {
tx, err = database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, "ip-a", nextMinute); err != nil {
t.Fatalf("CheckAndIncrementTx() error = %v", err)
}
if err := tx.Commit(); err != nil {
t.Fatalf("Commit() error = %v", err)
}
}

tx, err = database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, "ip-a", nextMinute); err != ErrRateLimitExceeded {
t.Fatalf("CheckAndIncrementTx() error = %v, want %v", err, ErrRateLimitExceeded)
}
_ = tx.Rollback()

for _, ip := range []string{"ip-b", "ip-c"} {
tx, err = database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, ip, nextMinute); err != nil {
t.Fatalf("CheckAndIncrementTx() error = %v", err)
}
if err := tx.Commit(); err != nil {
t.Fatalf("Commit() error = %v", err)
}
}

tx, err = database.BeginTx(ctx, nil)
if err != nil {
t.Fatalf("BeginTx() error = %v", err)
}
if err := limiter.CheckAndIncrementTx(ctx, tx, "ip-d", nextMinute); err != ErrRateLimitExceeded {
t.Fatalf("global limit error = %v, want %v", err, ErrRateLimitExceeded)
}
_ = tx.Rollback()
}

func TestLimiterCleanupExpired(t *testing.T) {
database := openLimiterTestDB(t)
limiter := New(Config{})
ctx := context.Background()
now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

if _, err := database.ExecContext(ctx, `INSERT INTO rate_limits(bucket, key, count, reset_at) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
"ip_minute", "expired", 1, now.Add(-time.Minute),
"ip_minute", "active", 1, now.Add(time.Minute),
); err != nil {
t.Fatalf("insert rate limits: %v", err)
}

if err := limiter.CleanupExpired(ctx, database, now); err != nil {
t.Fatalf("CleanupExpired() error = %v", err)
}

var count int
if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM rate_limits`).Scan(&count); err != nil {
t.Fatalf("query count: %v", err)
}
if count != 1 {
t.Fatalf("remaining buckets = %d, want 1", count)
}
}

func openLimiterTestDB(t *testing.T) *sql.DB {
t.Helper()
path := t.TempDir() + "/ratelimit.db"
database, err := clipdb.Open(path)
if err != nil {
t.Fatalf("Open() error = %v", err)
}
t.Cleanup(func() { _ = database.Close() })
if err := clipdb.Init(context.Background(), database); err != nil {
t.Fatalf("Init() error = %v", err)
}
return database
}
