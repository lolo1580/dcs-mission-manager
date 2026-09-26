# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — frontend build (Svelte + Vite)
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
# Output goes to backend/internal/api/dist (see vite.config.js)
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2 — backend build (Go), frontend embedded
# ---------------------------------------------------------------------------
# Keep the Go version in sync with backend/go.mod (Go 1.25): an older toolchain
# refuses to compile the module.
FROM golang:1.25-alpine AS backend
WORKDIR /src/backend
# go.sum is required: every dependency (pure-Go SQLite) is pinned.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# The built frontend comes from the previous stage; //go:embed picks it up.
COPY --from=frontend /src/backend/internal/api/dist ./internal/api/dist

ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/dcsmm ./cmd/dcsmm

# ---------------------------------------------------------------------------
# Stage 3 — minimal final image
# ---------------------------------------------------------------------------
FROM alpine:3.20
# ca-certificates: useful when an HTTPS basemap is used server-side.
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 dcsmm \
 && mkdir -p /data /app \
 && chown dcsmm /data

WORKDIR /app
COPY --from=backend /out/dcsmm /app/dcsmm

# The container holds ONLY the manager. DCS World runs on Windows and talks to
# it over the network (see docs/deployment.md).
ENV DCSMM_HTTP_ADDR=0.0.0.0:8080 \
    DCSMM_UDP_ADDR=0.0.0.0:7778 \
    DCSMM_TCP_ADDR=0.0.0.0:7779 \
    DCSMM_DB_PATH=/data/dcsmm.db \
    DCSMM_TILES_DIR=/app/tiles

EXPOSE 8080/tcp 7778/udp 7779/tcp
VOLUME ["/data"]

USER dcsmm
ENTRYPOINT ["/app/dcsmm"]
