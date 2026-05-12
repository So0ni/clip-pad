package noteshare

import "time"

const (
	ExpireOneDay   = "1d"
	ExpireSevenDay = "7d"
	ExpireThirty   = "30d"
	defaultIDLen   = 8
	maxIDRetries   = 5
	maxTitleBytes  = 512
)

type NoteShare struct {
	ID           string
	Title        string
	Content      string
	ContentBytes int
	ExpireMode   string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type CreateResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
