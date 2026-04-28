# ClipPad

ClipPad is a lightweight web tool with two focused features:

- **Paste-bin** for short-lived plain-text sharing with expiring links.
- **Simple Notepad** for temporary in-browser plain-text notes that disappear on refresh.

## Features

- Create plain-text pastes from `/`
- Short relative URLs in the form `/p/{id}`
- Expiration modes: `1 day`, `7 days`, `30 days`, `Burn after reading`
- Burn-after-reading reveal flow that deletes the paste immediately after reveal
- SQLite persistence for pastes and rate-limit counters
- Per-IP and global rate limits
- Total storage limit based on paste content bytes
- Simple English UI with mobile-friendly layout
- Pure front-end notepad with live characters, words, and lines counters
- Docker, docker-compose, and GitHub Actions support

## Unsupported features

ClipPad does **not** support:

- Editing existing pastes
- Deleting pastes manually
- Custom aliases
- Password-protected pastes
- Public paste listings
- Search
- User accounts
- Persistent storage for the notepad

## Local development

```bash
go mod download
go test ./...
go run ./cmd/server
```

The app listens on `:8080` by default.

## Docker

Build and run locally:

```bash
docker build -t clippad .
docker run -p 8080:8080 -v "$(pwd)/data:/data" clippad
```

The SQLite database defaults to `/data/clippad.db` inside the container.

## docker-compose

```bash
docker compose up -d --build
```

The included `docker-compose.yml` mounts `./data` to `/data` for persistence.

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CLIPPAD_ADDR` | `:8080` | HTTP listen address |
| `CLIPPAD_DB_PATH` | `/data/clippad.db` | SQLite database path |
| `CLIPPAD_MAX_PASTE_SIZE` | `1048576` | Maximum bytes per paste |
| `CLIPPAD_MAX_TOTAL_CONTENT_BYTES` | `1073741824` | Maximum total bytes stored across non-expired pastes |
| `CLIPPAD_RATE_LIMIT_PER_IP_PER_MINUTE` | `10` | Per-IP paste creation limit per minute |
| `CLIPPAD_RATE_LIMIT_PER_IP_PER_DAY` | `100` | Per-IP paste creation limit per day |
| `CLIPPAD_RATE_LIMIT_GLOBAL_PER_DAY` | `5000` | Global paste creation limit per day |
| `CLIPPAD_IP_HASH_SECRET` | generated at startup if empty | Secret used to hash IP addresses before writing rate-limit keys |
| `CLIPPAD_TRUST_CLOUDFLARE` | `true` | Trust `CF-Connecting-IP` only when the source IP matches a trusted proxy CIDR |
| `CLIPPAD_TRUST_PROXY_HEADERS` | `true` | Trust `X-Forwarded-For` only when the source IP matches a trusted proxy CIDR |
| `CLIPPAD_TRUSTED_PROXY_CIDRS` | empty | Comma-separated trusted proxy CIDRs. If empty, built-in Cloudflare CIDRs are used |

## SQLite persistence

ClipPad stores data in SQLite. Persist `/data` when running in Docker so `clippad.db` survives container restarts.

## Paste expiration behavior

- `1 day`, `7 days`, and `30 days` pastes store an `expires_at` timestamp.
- Expired pastes are deleted when they are accessed.
- A background cleanup task also removes expired pastes every hour.

## Burn after reading

Burn-after-reading pastes are not shown on the initial `GET /p/{id}` page. Users must trigger the reveal action, and the paste is deleted inside the reveal transaction immediately after the content is returned.

## Simple Notepad

The notepad at `/notepad` is fully front-end only:

- No back-end save API
- No `localStorage`, `sessionStorage`, or IndexedDB
- Refreshing the page clears the content
- Rich-text paste formatting is stripped to plain text

## Cloudflare and real IP handling

ClipPad can trust `CF-Connecting-IP` and `X-Forwarded-For`, but only when the request comes from a trusted proxy CIDR.

If `CLIPPAD_TRUSTED_PROXY_CIDRS` is empty, the app falls back to built-in Cloudflare IP ranges. For any other reverse proxy, set `CLIPPAD_TRUSTED_PROXY_CIDRS` explicitly.

> Important: if you use Cloudflare, ensure the origin only accepts traffic from Cloudflare or another trusted proxy layer. Otherwise proxy headers can be forged before they reach ClipPad.

## Rate limits

Default paste creation limits:

- 10 per IP per minute
- 100 per IP per day
- 5000 globally per day

Rate-limit keys use `sha256(CLIPPAD_IP_HASH_SECRET + real_ip)` so raw IPs are not stored in the database.

## Total storage limit

ClipPad sums `pastes.content_bytes` for all currently stored pastes. Before each paste creation, it removes expired rows and rejects new writes once the configured total byte limit is reached.

## Security notes

- Paste content is always rendered as plain text through `html/template`
- Front-end updates use `textContent`, never `innerHTML`
- Logs omit paste content and request bodies
- `robots.txt` disallows crawling and pages include `noindex,nofollow`
- The server uses HTTP timeouts to reduce abuse surface
- Set `CLIPPAD_IP_HASH_SECRET` in production instead of relying on the generated temporary secret

## GitHub Actions and GHCR

`.github/workflows/docker-build.yml` builds the Docker image for pull requests, and builds plus pushes to GHCR on pushes to `main` and tags matching `v*`.

The published image name is:

```text
ghcr.io/${GITHUB_REPOSITORY_OWNER}/clippad
```
