package config

import (
"crypto/rand"
"encoding/hex"
"fmt"
"log"
"net/netip"
"os"
"strconv"
"strings"
)

const (
defaultAddr                    = ":8080"
defaultDBPath                  = "/data/clippad.db"
defaultMaxPasteSize            = 1048576
defaultMaxTotalContentBytes    = 1073741824
defaultRateLimitPerIPPerMinute = 10
defaultRateLimitPerIPPerDay    = 100
defaultRateLimitGlobalPerDay   = 5000
)

type Config struct {
Addr                    string
DBPath                  string
MaxPasteSize            int
MaxTotalContentBytes    int64
RateLimitPerIPPerMinute int
RateLimitPerIPPerDay    int
RateLimitGlobalPerDay   int
IPHashSecret            string
TrustCloudflare         bool
TrustProxyHeaders       bool
TrustedProxyCIDRs       []netip.Prefix
}

func Load() (Config, error) {
cfg := Config{
Addr:                    getEnv("CLIPPAD_ADDR", defaultAddr),
DBPath:                  getEnv("CLIPPAD_DB_PATH", defaultDBPath),
TrustCloudflare:         getEnvBool("CLIPPAD_TRUST_CLOUDFLARE", true),
TrustProxyHeaders:       getEnvBool("CLIPPAD_TRUST_PROXY_HEADERS", true),
IPHashSecret:            strings.TrimSpace(os.Getenv("CLIPPAD_IP_HASH_SECRET")),
}

var err error
if cfg.MaxPasteSize, err = getEnvInt("CLIPPAD_MAX_PASTE_SIZE", defaultMaxPasteSize); err != nil {
return Config{}, err
}

maxTotalContentBytes, err := getEnvInt64("CLIPPAD_MAX_TOTAL_CONTENT_BYTES", defaultMaxTotalContentBytes)
if err != nil {
return Config{}, err
}
cfg.MaxTotalContentBytes = maxTotalContentBytes

if cfg.RateLimitPerIPPerMinute, err = getEnvInt("CLIPPAD_RATE_LIMIT_PER_IP_PER_MINUTE", defaultRateLimitPerIPPerMinute); err != nil {
return Config{}, err
}
if cfg.RateLimitPerIPPerDay, err = getEnvInt("CLIPPAD_RATE_LIMIT_PER_IP_PER_DAY", defaultRateLimitPerIPPerDay); err != nil {
return Config{}, err
}
if cfg.RateLimitGlobalPerDay, err = getEnvInt("CLIPPAD_RATE_LIMIT_GLOBAL_PER_DAY", defaultRateLimitGlobalPerDay); err != nil {
return Config{}, err
}
if cfg.TrustedProxyCIDRs, err = parseTrustedCIDRs(os.Getenv("CLIPPAD_TRUSTED_PROXY_CIDRS")); err != nil {
return Config{}, err
}

if cfg.IPHashSecret == "" {
cfg.IPHashSecret, err = generateEphemeralSecret()
if err != nil {
return Config{}, fmt.Errorf("generate IP hash secret: %w", err)
}
log.Print("warning: CLIPPAD_IP_HASH_SECRET is empty; generated an ephemeral secret for this process")
}

return cfg, nil
}

func parseTrustedCIDRs(value string) ([]netip.Prefix, error) {
value = strings.TrimSpace(value)
if value == "" {
return nil, nil
}

parts := strings.Split(value, ",")
prefixes := make([]netip.Prefix, 0, len(parts))
for _, part := range parts {
part = strings.TrimSpace(part)
if part == "" {
continue
}
prefix, err := netip.ParsePrefix(part)
if err != nil {
return nil, fmt.Errorf("parse CLIPPAD_TRUSTED_PROXY_CIDRS: %w", err)
}
prefixes = append(prefixes, prefix)
}
return prefixes, nil
}

func generateEphemeralSecret() (string, error) {
buf := make([]byte, 32)
if _, err := rand.Read(buf); err != nil {
return "", err
}
return hex.EncodeToString(buf), nil
}

func getEnv(key, fallback string) string {
value := strings.TrimSpace(os.Getenv(key))
if value == "" {
return fallback
}
return value
}

func getEnvInt(key string, fallback int) (int, error) {
value := strings.TrimSpace(os.Getenv(key))
if value == "" {
return fallback, nil
}
parsed, err := strconv.Atoi(value)
if err != nil {
return 0, fmt.Errorf("parse %s: %w", key, err)
}
return parsed, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
value := strings.TrimSpace(os.Getenv(key))
if value == "" {
return fallback, nil
}
parsed, err := strconv.ParseInt(value, 10, 64)
if err != nil {
return 0, fmt.Errorf("parse %s: %w", key, err)
}
return parsed, nil
}

func getEnvBool(key string, fallback bool) bool {
value := strings.TrimSpace(os.Getenv(key))
if value == "" {
return fallback
}
parsed, err := strconv.ParseBool(value)
if err != nil {
return fallback
}
return parsed
}
