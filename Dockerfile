# =============================================================================
# Dockerfile for capabilities-registry (multi-stage build)
# =============================================================================

# Stage 1: Build
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /capabilities-registry \
    ./cmd/capabilities-registry

# Stage 2: Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /capabilities-registry /app/capabilities-registry

# Copy migrations and config
COPY migrations/ /app/migrations/
COPY config/ /app/config/

# Expose HTTP health port
EXPOSE 8080

# Default environment variables
ENV COMMS_EMBED=true \
    COMMS_PORT=4222 \
    HTTP_PORT=8080 \
    LOG_LEVEL=info \
    RUN_MIGRATIONS=false \
    MIGRATION_PATH=/app/migrations

ENTRYPOINT ["/app/capabilities-registry"]
