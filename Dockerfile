FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /out/data && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/clippad ./cmd/server

FROM scratch
WORKDIR /app
COPY --from=builder /out/clippad /app/clippad
COPY --from=builder /app/web /app/web
COPY --from=builder /out/data /data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/clippad"]
