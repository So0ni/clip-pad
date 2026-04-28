package ratelimit

import (
"context"
"database/sql"
"errors"
"time"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type Config struct {
PerIPPerMinute int
PerIPPerDay    int
GlobalPerDay   int
}

type Limiter struct {
cfg Config
}

type execer interface {
ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func New(cfg Config) *Limiter {
return &Limiter{cfg: cfg}
}

func (l *Limiter) CheckAndIncrementTx(ctx context.Context, tx *sql.Tx, ipHash string, now time.Time) error {
rules := []struct {
bucket  string
key     string
limit   int
resetAt time.Time
}{
{bucket: "ip_minute", key: ipHash, limit: l.cfg.PerIPPerMinute, resetAt: minuteReset(now)},
{bucket: "ip_day", key: ipHash, limit: l.cfg.PerIPPerDay, resetAt: dayReset(now)},
{bucket: "global_day", key: "global", limit: l.cfg.GlobalPerDay, resetAt: dayReset(now)},
}

for _, rule := range rules {
if rule.limit <= 0 {
continue
}

count, resetAt, found, err := loadBucket(ctx, tx, rule.bucket, rule.key)
if err != nil {
return err
}
if found && !resetAt.After(now.UTC()) {
if _, err := tx.ExecContext(ctx, `DELETE FROM rate_limits WHERE bucket = ? AND key = ?`, rule.bucket, rule.key); err != nil {
return err
}
found = false
count = 0
}
if count+1 > rule.limit {
return ErrRateLimitExceeded
}
if found {
if _, err := tx.ExecContext(ctx, `UPDATE rate_limits SET count = ?, reset_at = ? WHERE bucket = ? AND key = ?`, count+1, rule.resetAt.UTC(), rule.bucket, rule.key); err != nil {
return err
}
continue
}
if _, err := tx.ExecContext(ctx, `INSERT INTO rate_limits(bucket, key, count, reset_at) VALUES (?, ?, ?, ?)`, rule.bucket, rule.key, 1, rule.resetAt.UTC()); err != nil {
return err
}
}

return nil
}

func (l *Limiter) CleanupExpired(ctx context.Context, db execer, now time.Time) error {
_, err := db.ExecContext(ctx, `DELETE FROM rate_limits WHERE reset_at < ?`, now.UTC())
return err
}

func loadBucket(ctx context.Context, tx *sql.Tx, bucket, key string) (int, time.Time, bool, error) {
var count int
var resetAt time.Time
err := tx.QueryRowContext(ctx, `SELECT count, reset_at FROM rate_limits WHERE bucket = ? AND key = ?`, bucket, key).Scan(&count, &resetAt)
if errors.Is(err, sql.ErrNoRows) {
return 0, time.Time{}, false, nil
}
if err != nil {
return 0, time.Time{}, false, err
}
return count, resetAt, true, nil
}

func minuteReset(now time.Time) time.Time {
return now.UTC().Truncate(time.Minute).Add(time.Minute)
}

func dayReset(now time.Time) time.Time {
now = now.UTC()
year, month, day := now.Date()
return time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC)
}
