ARG VERSION=dev

# Build the web UI first: Vite writes into backend/internal/api/dist, which the
# Go binary embeds. The final image is a small Alpine runtime with no Node or Go.
FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci
COPY frontend ./frontend
RUN cd frontend && npm run build

FROM golang:1.26-alpine AS backend
ARG VERSION
RUN apk add --no-cache ca-certificates
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/backend/internal/api/dist ./internal/api/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/dcsmanager ./cmd/dcsmanager

FROM alpine:3.21
ARG VERSION
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 -h /app dcs \
    && mkdir -p /data \
    && chown -R dcs:dcs /data
WORKDIR /app
COPY --chown=dcs:dcs --from=backend /out/dcsmanager /app/dcsmanager
USER dcs

LABEL org.opencontainers.image.title="DCS Manager web" \
      org.opencontainers.image.description="Multi-user server for DCS Manager" \
      org.opencontainers.image.version="${VERSION}"

ENV DCSMANAGER_MODE=server \
    DCSMANAGER_HTTP_ADDR=0.0.0.0:8080 \
    DCSMANAGER_DB_PATH=/data/dcsmanager.db

EXPOSE 8080
VOLUME ["/data"]
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/app/dcsmanager", "server"]
