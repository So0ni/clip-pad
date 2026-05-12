package noteshare

import (
	"context"
	"strings"
	"testing"
	"time"

	clipdb "github.com/So0ni/clip-pad/internal/db"
	"github.com/So0ni/clip-pad/internal/ratelimit"
)

func TestCreateAndReadNoteShare(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	created, err := service.Create(context.Background(), "Roadmap", "ship snapshot sharing", ExpireSevenDay, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if !created.ExpiresAt.Equal(service.now().UTC().Add(7 * 24 * time.Hour)) {
		t.Fatalf("ExpiresAt = %v, want seven days from now", created.ExpiresAt)
	}

	loaded, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Title != "Roadmap" {
		t.Fatalf("Title = %q, want Roadmap", loaded.Title)
	}
	if loaded.Content != "ship snapshot sharing" {
		t.Fatalf("Content = %q, want snapshot content", loaded.Content)
	}
}

func TestCreateNoteShareValidation(t *testing.T) {
	service := newTestService(t, Config{MaxContentSize: 32, MaxTotalContentBytes: 128, IPHashSecret: "secret", RateLimit: ratelimit.Config{PerIPPerMinute: 10, PerIPPerDay: 100, GlobalPerDay: 1000}})

	if _, err := service.Create(context.Background(), "", "", ExpireOneDay, "127.0.0.1"); err != ErrContentRequired {
		t.Fatalf("empty content error = %v, want %v", err, ErrContentRequired)
	}
	if _, err := service.Create(context.Background(), "", " \n\t ", ExpireOneDay, "127.0.0.1"); err != ErrContentRequired {
		t.Fatalf("whitespace content error = %v, want %v", err, ErrContentRequired)
	}
	if _, err := service.Create(context.Background(), "", strings.Repeat("a", 33), ExpireOneDay, "127.0.0.1"); err != ErrContentTooLarge {
		t.Fatalf("content too large error = %v, want %v", err, ErrContentTooLarge)
	}
	if _, err := service.Create(context.Background(), "", "hello", "burn", "127.0.0.1"); err != ErrInvalidExpireMode {
		t.Fatalf("burn expire error = %v, want %v", err, ErrInvalidExpireMode)
	}
}

func TestLongASCIINameIsTruncated(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	created, err := service.Create(context.Background(), strings.Repeat("a", 201), "hello", ExpireOneDay, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Title) != 80 {
		t.Fatalf("Title length = %d, want 80", len(created.Title))
	}
}

func TestExpiredNoteShareReturnsNotFoundAndDeletes(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	created, err := service.Create(context.Background(), "", "expired", ExpireOneDay, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.now = func() time.Time { return now.Add(25 * time.Hour) }

	if _, err := service.Get(context.Background(), created.ID); err != ErrNotFound {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}

	var count int
	if err := service.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM note_shares WHERE id = ?`, created.ID).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expired note share count = %d, want 0", count)
	}
}

func newTestService(t *testing.T, cfg Config) *Service {
	t.Helper()
	path := t.TempDir() + "/clippad.db"
	database, err := clipdb.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := clipdb.Init(context.Background(), database); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	service := NewService(database, cfg)
	service.now = func() time.Time { return time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC) }
	return service
}

func defaultTestConfig() Config {
	return Config{
		MaxContentSize:       1024,
		MaxTotalContentBytes: 8192,
		IPHashSecret:         "secret",
		RateLimit:            ratelimit.Config{PerIPPerMinute: 10, PerIPPerDay: 100, GlobalPerDay: 1000},
	}
}
