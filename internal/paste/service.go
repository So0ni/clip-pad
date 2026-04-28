package paste

import (
"context"
"crypto/sha256"
"database/sql"
"encoding/hex"
"errors"
"fmt"
"strings"
"time"

"github.com/So0ni/clip-pad/internal/ratelimit"
"github.com/So0ni/clip-pad/internal/utils"
)

var (
ErrContentRequired      = errors.New("content is required")
ErrContentTooLarge      = errors.New("content exceeds the maximum size")
ErrInvalidExpireMode    = errors.New("invalid expire value")
ErrNotFound             = errors.New("paste not found")
ErrStorageLimitReached  = errors.New("Storage limit reached")
ErrNotBurnPaste         = errors.New("paste is not burn-after-reading")
ErrIDGenerationFailed   = errors.New("failed to generate a unique paste ID")
)

type Config struct {
MaxPasteSize         int
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

func (s *Service) Create(ctx context.Context, content, expireMode, realIP string) (*Paste, error) {
if strings.TrimSpace(content) == "" {
return nil, ErrContentRequired
}

contentBytes := len([]byte(content))
if contentBytes > s.cfg.MaxPasteSize {
return nil, ErrContentTooLarge
}

now := s.now().UTC()
expiresAt, burnAfterRead, err := CalculateExpiry(now, expireMode)
if err != nil {
return nil, err
}

tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, fmt.Errorf("begin create paste transaction: %w", err)
}
defer tx.Rollback()

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

created := &Paste{
Content:       content,
ContentBytes:  contentBytes,
ExpireMode:    expireMode,
ExpiresAt:     expiresAt,
BurnAfterRead: burnAfterRead,
CreatedAt:     now,
}

for attempt := 0; attempt < maxIDRetries; attempt++ {
id, err := s.generateID(defaultIDLen)
if err != nil {
return nil, fmt.Errorf("generate paste ID: %w", err)
}
created.ID = id
_, err = tx.ExecContext(ctx, `
INSERT INTO pastes(id, content, content_bytes, expire_mode, expires_at, burn_after_read, created_at, viewed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
`, created.ID, created.Content, created.ContentBytes, created.ExpireMode, created.ExpiresAt, boolToInt(created.BurnAfterRead), created.CreatedAt)
if err == nil {
if commitErr := tx.Commit(); commitErr != nil {
return nil, fmt.Errorf("commit create paste transaction: %w", commitErr)
}
return created, nil
}
if !isUniqueConstraint(err) {
return nil, fmt.Errorf("insert paste: %w", err)
}
}

return nil, ErrIDGenerationFailed
}

func (s *Service) Get(ctx context.Context, id string) (*Paste, error) {
if !isValidID(id) {
return nil, ErrNotFound
}

paste, err := s.loadPaste(ctx, s.db, id)
if err != nil {
return nil, err
}

now := s.now().UTC()
if isExpired(paste.ExpiresAt, now) {
if _, err := s.db.ExecContext(ctx, `DELETE FROM pastes WHERE id = ?`, id); err != nil {
return nil, fmt.Errorf("delete expired paste: %w", err)
}
return nil, ErrNotFound
}

if !paste.BurnAfterRead && paste.ViewedAt == nil {
viewedAt := now
if _, err := s.db.ExecContext(ctx, `UPDATE pastes SET viewed_at = ? WHERE id = ? AND viewed_at IS NULL`, viewedAt, id); err != nil {
return nil, fmt.Errorf("update paste viewed_at: %w", err)
}
paste.ViewedAt = &viewedAt
}

return paste, nil
}

func (s *Service) Reveal(ctx context.Context, id string) (*Paste, error) {
if !isValidID(id) {
return nil, ErrNotFound
}

tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
return nil, fmt.Errorf("begin reveal transaction: %w", err)
}
defer tx.Rollback()

paste, err := s.loadPaste(ctx, tx, id)
if err != nil {
return nil, err
}
if isExpired(paste.ExpiresAt, s.now().UTC()) {
if _, err := tx.ExecContext(ctx, `DELETE FROM pastes WHERE id = ?`, id); err != nil {
return nil, fmt.Errorf("delete expired paste: %w", err)
}
return nil, ErrNotFound
}
if !paste.BurnAfterRead {
return nil, ErrNotBurnPaste
}
if _, err := tx.ExecContext(ctx, `DELETE FROM pastes WHERE id = ?`, id); err != nil {
return nil, fmt.Errorf("delete burn paste: %w", err)
}
if err := tx.Commit(); err != nil {
return nil, fmt.Errorf("commit reveal transaction: %w", err)
}
return paste, nil
}

func (s *Service) CleanupExpiredPastes(ctx context.Context) error {
return s.cleanupExpiredPastesTx(ctx, s.db, s.now().UTC())
}

func (s *Service) CleanupExpiredRateLimits(ctx context.Context) error {
if err := s.limiter.CleanupExpired(ctx, s.db, s.now().UTC()); err != nil {
return fmt.Errorf("cleanup expired rate limits: %w", err)
}
return nil
}

type queryer interface {
QueryRowContext(context.Context, string, ...any) *sql.Row
}

type execQueryer interface {
queryer
ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) loadPaste(ctx context.Context, db queryer, id string) (*Paste, error) {
var paste Paste
var expiresAt sql.NullTime
var viewedAt sql.NullTime
var burnAfterRead int
err := db.QueryRowContext(ctx, `
SELECT id, content, content_bytes, expire_mode, expires_at, burn_after_read, created_at, viewed_at
FROM pastes WHERE id = ?
`, id).Scan(
&paste.ID,
&paste.Content,
&paste.ContentBytes,
&paste.ExpireMode,
&expiresAt,
&burnAfterRead,
&paste.CreatedAt,
&viewedAt,
)
if errors.Is(err, sql.ErrNoRows) {
return nil, ErrNotFound
}
if err != nil {
return nil, fmt.Errorf("load paste: %w", err)
}
if expiresAt.Valid {
value := expiresAt.Time.UTC()
paste.ExpiresAt = &value
}
if viewedAt.Valid {
value := viewedAt.Time.UTC()
paste.ViewedAt = &value
}
paste.CreatedAt = paste.CreatedAt.UTC()
paste.BurnAfterRead = burnAfterRead == 1
return &paste, nil
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
if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(content_bytes), 0) FROM pastes`).Scan(&total); err != nil {
return 0, fmt.Errorf("sum paste bytes: %w", err)
}
if !total.Valid {
return 0, nil
}
return total.Int64, nil
}

func CalculateExpiry(now time.Time, mode string) (*time.Time, bool, error) {
now = now.UTC()
switch mode {
case ExpireOneDay:
expiresAt := now.Add(24 * time.Hour)
return &expiresAt, false, nil
case ExpireSevenDay:
expiresAt := now.Add(7 * 24 * time.Hour)
return &expiresAt, false, nil
case ExpireThirty:
expiresAt := now.Add(30 * 24 * time.Hour)
return &expiresAt, false, nil
case ExpireBurn:
return nil, true, nil
default:
return nil, false, ErrInvalidExpireMode
}
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

func isExpired(expiresAt *time.Time, now time.Time) bool {
return expiresAt != nil && !expiresAt.After(now.UTC())
}

func isUniqueConstraint(err error) bool {
return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func boolToInt(value bool) int {
if value {
return 1
}
return 0
}
