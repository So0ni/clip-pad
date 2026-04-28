package paste

import "time"

const (
	ExpireOneDay   = "1d"
	ExpireSevenDay = "7d"
	ExpireThirty   = "30d"
	ExpireBurn     = "burn"
	defaultIDLen   = 8
	maxIDRetries   = 5

	ThemeWarm = "warm"
	ThemeBlue = "blue"
	ThemeMilk = "milk"
)

type Paste struct {
	ID            string
	Content       string
	ContentBytes  int
	ExpireMode    string
	ExpiresAt     *time.Time
	BurnAfterRead bool
	CreatedAt     time.Time
	ViewedAt      *time.Time
	Theme         string
}

type CreateResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
