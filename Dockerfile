# =============================================================================
# Dockerfile for more0ai/registry (multi-stage, non-root, production-ready)
# Image: ghcr.io/more0ai/registry
# =============================================================================

# Stage 1: Build
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary; output name "registry" for CLI (registry serve, registry migrate up)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /registry \
    ./cmd/registry

# Stage 2: Runtime (distroless would require static binary + no shell; Alpine is smaller than full debian and supports non-root)
FROM alpine:3.20

# Non-root user for running the registry
RUN addgroup -g 1000 -S appgroup && adduser -u 1000 -S appuser -G appgroup

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Copy binary (named registry for user-facing commands in image)
COPY --from=builder /registry /app/registry

# License and notice (required for BSL 1.1 and OCI)
COPY LICENSE /app/LICENSE
COPY NOTICE /app/NOTICE

# Migrations, config, and capability metadata (for seeding system.registry etc.)
COPY migrations/ /app/migrations/
COPY config/ /app/config/
COPY capabilities/ /app/capabilities/

# Owned by appuser
RUN chown -R appuser:appgroup /app

USER appuser

# Default HTTP listen address (override with REGISTRY_HTTP_ADDR or HTTP_PORT)
ENV REGISTRY_HTTP_ADDR=0.0.0.0:8080 \
    HTTP_PORT=8080 \
    LOG_LEVEL=info \
    RUN_MIGRATIONS=false \
    MIGRATION_PATH=/app/migrations

EXPOSE 8080

# OCI labels (VERSION passed as build-arg in CI: --build-arg VERSION=1.2.3)
ARG VERSION
LABEL org.opencontainers.image.source="https://github.com/more0ai/registry" \
      org.opencontainers.image.title="more0ai/registry" \
      org.opencontainers.image.licenses="BSL-1.1" \
      org.opencontainers.image.version="${VERSION}"

# Default: serve. Override to run migrations: registry migrate up
ENTRYPOINT ["/app/registry"]
CMD ["serve"]

# -----------------------------------------------------------------------------
# Stage: test (run unit + integration tests inside container)
# Usage: docker compose run --rm test
# Requires: Postgres reachable (POSTGRES_HOST, etc.); creates registry_test via ensure-db.
# -----------------------------------------------------------------------------
FROM builder AS test

RUN apk add --no-cache bash

# Binary from builder is at /registry; ensure-db creates registry_test on same host as DATABASE_URL
CMD ["sh", "-c", "/registry ensure-db && go test ./... && go test -tags=integration ./..."]
