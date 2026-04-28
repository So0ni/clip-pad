FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/clippad ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser && mkdir -p /data /app && chown -R appuser:appuser /data /app
WORKDIR /app
COPY --from=builder /out/clippad /app/clippad
COPY --from=builder /app/web /app/web
EXPOSE 8080
VOLUME ["/data"]
USER appuser
ENTRYPOINT ["/app/clippad"]
