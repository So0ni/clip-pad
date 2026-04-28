package paste

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	clipdb "github.com/So0ni/clip-pad/internal/db"
	"github.com/So0ni/clip-pad/internal/ratelimit"
)

func TestCalculateExpiry(t *testing.T) {
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		mode     string
		wantTime *time.Time
		wantBurn bool
		wantErr  error
	}{
		{name: "1d", mode: ExpireOneDay, wantTime: timePtr(now.Add(24 * time.Hour))},
		{name: "7d", mode: ExpireSevenDay, wantTime: timePtr(now.Add(7 * 24 * time.Hour))},
		{name: "30d", mode: ExpireThirty, wantTime: timePtr(now.Add(30 * 24 * time.Hour))},
		{name: "burn", mode: ExpireBurn, wantBurn: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotTime, gotBurn, err := CalculateExpiry(now, test.mode)
			if err != nil {
				t.Fatalf("CalculateExpiry() error = %v", err)
			}
			if gotBurn != test.wantBurn {
				t.Fatalf("burn = %v, want %v", gotBurn, test.wantBurn)
			}
			if test.wantTime == nil {
				if gotTime != nil {
					t.Fatalf("expiresAt = %v, want nil", gotTime)
				}
				return
			}
			if gotTime == nil || !gotTime.Equal(*test.wantTime) {
				t.Fatalf("expiresAt = %v, want %v", gotTime, *test.wantTime)
			}
		})
	}
}

func TestCreatePasteValidation(t *testing.T) {
	service := newTestService(t, Config{MaxPasteSize: 32, MaxTotalContentBytes: 128, IPHashSecret: "secret", RateLimit: ratelimit.Config{PerIPPerMinute: 10, PerIPPerDay: 100, GlobalPerDay: 1000}})

	if _, err := service.Create(context.Background(), "", ExpireOneDay, "127.0.0.1"); err != ErrContentRequired {
		t.Fatalf("empty content error = %v, want %v", err, ErrContentRequired)
	}
	if _, err := service.Create(context.Background(), " \n\t ", ExpireOneDay, "127.0.0.1"); err != ErrContentRequired {
		t.Fatalf("whitespace content error = %v, want %v", err, ErrContentRequired)
	}
	if _, err := service.Create(context.Background(), strings.Repeat("a", 33), ExpireOneDay, "127.0.0.1"); err != ErrContentTooLarge {
		t.Fatalf("content too large error = %v, want %v", err, ErrContentTooLarge)
	}
	if _, err := service.Create(context.Background(), "hello", "bad", "127.0.0.1"); err != ErrInvalidExpireMode {
		t.Fatalf("invalid expire error = %v, want %v", err, ErrInvalidExpireMode)
	}
}

func TestCreateAndReadPaste(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	created, err := service.Create(context.Background(), "hello world", ExpireOneDay, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}

	loaded, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Content != "hello world" {
		t.Fatalf("content = %q, want %q", loaded.Content, "hello world")
	}
	if loaded.ViewedAt == nil {
		t.Fatal("ViewedAt was not set for normal paste")
	}
}

func TestExpiredPasteReturnsNotFoundAndDeletes(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	created, err := service.Create(context.Background(), "expired", ExpireOneDay, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.now = func() time.Time { return now.Add(25 * time.Hour) }

	if _, err := service.Get(context.Background(), created.ID); err != ErrNotFound {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}

	var count int
	if err := service.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pastes WHERE id = ?`, created.ID).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expired paste count = %d, want 0", count)
	}
}

func TestMissingPasteReturnsNotFound(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	if _, err := service.Get(context.Background(), "abcdefgh"); err != ErrNotFound {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}
}

func TestBurnAfterReadingRevealDeletesPaste(t *testing.T) {
	service := newTestService(t, defaultTestConfig())
	created, err := service.Create(context.Background(), "secret text", ExpireBurn, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !loaded.BurnAfterRead {
		t.Fatal("burn paste not marked as burn-after-read")
	}

	revealed, err := service.Reveal(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}
	if revealed.Content != "secret text" {
		t.Fatalf("revealed content = %q, want %q", revealed.Content, "secret text")
	}

	if _, err := service.Get(context.Background(), created.ID); err != ErrNotFound {
		t.Fatalf("Get() after reveal error = %v, want %v", err, ErrNotFound)
	}
}

func TestStorageLimitAndCleanup(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.MaxTotalContentBytes = 10
	service := newTestService(t, cfg)
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if _, err := service.Create(context.Background(), "12345", ExpireOneDay, "127.0.0.1"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Create(context.Background(), "67890", ExpireOneDay, "127.0.0.2"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Create(context.Background(), "x", ExpireOneDay, "127.0.0.3"); err != ErrStorageLimitReached {
		t.Fatalf("Create() error = %v, want %v", err, ErrStorageLimitReached)
	}

	service.now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := service.Create(context.Background(), "x", ExpireOneDay, "127.0.0.4"); err != nil {
		t.Fatalf("Create() after cleanup error = %v", err)
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
		MaxPasteSize:         1024,
		MaxTotalContentBytes: 8192,
		IPHashSecret:         "secret",
		RateLimit:            ratelimit.Config{PerIPPerMinute: 10, PerIPPerDay: 100, GlobalPerDay: 1000},
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
