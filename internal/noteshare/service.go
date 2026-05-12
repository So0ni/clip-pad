package noteshare

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/So0ni/clip-pad/internal/ratelimit"
	"github.com/So0ni/clip-pad/internal/utils"
)

var (
	ErrTitleTooLarge       = errors.New("title exceeds the maximum size")
	ErrContentRequired     = errors.New("content is required")
	ErrContentTooLarge     = errors.New("content exceeds the maximum size")
	ErrInvalidExpireMode   = errors.New("invalid expire value")
	ErrNotFound            = errors.New("note share not found")
	ErrStorageLimitReached = errors.New("Storage limit reached")
	ErrIDGenerationFailed  = errors.New("failed to generate a unique note share ID")
)

type Config struct {
	MaxContentSize       int
	MaxTotalContentBytes int64
	IPHashSecret         string
	RateLimit            ratelimit.Config
}

type Service struct {
	db         *sql.DB
	cfg        Config
	limiter    *ratelimit.Limiter
	generateID func(int) (string, error)
	now        func() time.Time
}

func NewService(db *sql.DB, cfg Config) *Service {
	return &Service{
		db:         db,
		cfg:        cfg,
		limiter:    ratelimit.New(cfg.RateLimit),
		generateID: utils.GenerateID,
		now:        time.Now,
	}
}

func (s *Service) Create(ctx context.Context, title, content, expireMode, realIP string) (*NoteShare, error) {
	if strings.TrimSpace(content) == "" {
		return nil, ErrContentRequired
	}

	title = normalizeTitle(title, content)
	if len([]byte(title)) > maxTitleBytes {
		return nil, ErrTitleTooLarge
	}

	contentBytes := len([]byte(content))
	if contentBytes > s.cfg.MaxContentSize {
		return nil, ErrContentTooLarge
	}

	now := s.now().UTC()
	expiresAt, err := calculateExpiry(now, expireMode)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create note share transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.cleanupExpiredNoteSharesTx(ctx, tx, now); err != nil {
		return nil, err
	}
	if err := s.cleanupExpiredPastesTx(ctx, tx, now); err != nil {
		return nil, err
	}
	if err := s.limiter.CleanupExpired(ctx, tx, now); err != nil {
		return nil, fmt.Errorf("cleanup expired rate limits: %w", err)
	}

	currentBytes, err := s.totalContentBytesTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	if currentBytes+int64(contentBytes) > s.cfg.MaxTotalContentBytes {
		return nil, ErrStorageLimitReached
	}

	ipHash := hashIP(s.cfg.IPHashSecret, realIP)
	if err := s.limiter.CheckAndIncrementTx(ctx, tx, ipHash, now); err != nil {
		return nil, err
	}

	created := &NoteShare{
		Title:        title,
		Content:      content,
		ContentBytes: contentBytes,
		ExpireMode:   expireMode,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
	}

	for attempt := 0; attempt < maxIDRetries; attempt++ {
		id, err := s.generateID(defaultIDLen)
		if err != nil {
			return nil, fmt.Errorf("generate note share ID: %w", err)
		}
		created.ID = id
		_, err = tx.ExecContext(ctx, `
INSERT INTO note_shares(id, title, content, content_bytes, expire_mode, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, created.ID, created.Title, created.Content, created.ContentBytes, created.ExpireMode, created.ExpiresAt, created.CreatedAt)
		if err == nil {
			if commitErr := tx.Commit(); commitErr != nil {
				return nil, fmt.Errorf("commit create note share transaction: %w", commitErr)
			}
			return created, nil
		}
		if !isUniqueConstraint(err) {
			return nil, fmt.Errorf("insert note share: %w", err)
		}
	}

	return nil, ErrIDGenerationFailed
}

func (s *Service) Get(ctx context.Context, id string) (*NoteShare, error) {
	if !isValidID(id) {
		return nil, ErrNotFound
	}

	share, err := s.loadNoteShare(ctx, s.db, id)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	if !share.ExpiresAt.After(now) {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM note_shares WHERE id = ?`, id); err != nil {
			return nil, fmt.Errorf("delete expired note share: %w", err)
		}
		return nil, ErrNotFound
	}

	return share, nil
}

func (s *Service) CleanupExpiredNoteShares(ctx context.Context) error {
	return s.cleanupExpiredNoteSharesTx(ctx, s.db, s.now().UTC())
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type execQueryer interface {
	queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) loadNoteShare(ctx context.Context, db queryer, id string) (*NoteShare, error) {
	var share NoteShare
	err := db.QueryRowContext(ctx, `
SELECT id, title, content, content_bytes, expire_mode, expires_at, created_at
FROM note_shares WHERE id = ?
`, id).Scan(
		&share.ID,
		&share.Title,
		&share.Content,
		&share.ContentBytes,
		&share.ExpireMode,
		&share.ExpiresAt,
		&share.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load note share: %w", err)
	}
	share.ExpiresAt = share.ExpiresAt.UTC()
	share.CreatedAt = share.CreatedAt.UTC()
	return &share, nil
}

func (s *Service) cleanupExpiredNoteSharesTx(ctx context.Context, db execQueryer, now time.Time) error {
	_, err := db.ExecContext(ctx, `DELETE FROM note_shares WHERE expires_at < ?`, now.UTC())
	if err != nil {
		return fmt.Errorf("cleanup expired note shares: %w", err)
	}
	return nil
}

func (s *Service) cleanupExpiredPastesTx(ctx context.Context, db execQueryer, now time.Time) error {
	_, err := db.ExecContext(ctx, `DELETE FROM pastes WHERE expires_at IS NOT NULL AND expires_at < ?`, now.UTC())
	if err != nil {
		return fmt.Errorf("cleanup expired pastes: %w", err)
	}
	return nil
}

func (s *Service) totalContentBytesTx(ctx context.Context, db queryer) (int64, error) {
	var total sql.NullInt64
	if err := db.QueryRowContext(ctx, `
SELECT
  COALESCE((SELECT SUM(content_bytes) FROM pastes), 0) +
  COALESCE((SELECT SUM(content_bytes) FROM note_shares), 0)
`).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum stored content bytes: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func calculateExpiry(now time.Time, mode string) (time.Time, error) {
	now = now.UTC()
	switch mode {
	case ExpireOneDay:
		return now.Add(24 * time.Hour), nil
	case ExpireSevenDay:
		return now.Add(7 * 24 * time.Hour), nil
	case ExpireThirty:
		return now.Add(30 * 24 * time.Hour), nil
	default:
		return time.Time{}, ErrInvalidExpireMode
	}
}

func normalizeTitle(title, content string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				title = line
				break
			}
		}
	}
	if title == "" {
		return "Untitled"
	}
	return truncateRunes(title, 80)
}

func truncateRunes(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes-3]) + "..."
}

func hashIP(secret, realIP string) string {
	if strings.TrimSpace(realIP) == "" {
		realIP = "unknown"
	}
	sum := sha256.Sum256([]byte(secret + realIP))
	return hex.EncodeToString(sum[:])
}

func isValidID(id string) bool {
	if len(id) != defaultIDLen {
		return false
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint")
}
