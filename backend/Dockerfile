### Stage 1 — build
FROM golang:1.25-alpine AS builder
WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o server ./cmd/server

### Stage 2 — runtime
FROM alpine:3.20
RUN apk add --no-cache wget ca-certificates tzdata

# Non-root user
RUN addgroup -g 1000 orchid && \
    adduser -D -u 1000 -G orchid orchid

WORKDIR /app
RUN mkdir -p /app/data /app/uploads /app/logs && \
    chown -R orchid:orchid /app

COPY --from=builder /app/server .

USER orchid

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3001/health || exit 1

EXPOSE 3001
ENTRYPOINT ["./server"]
