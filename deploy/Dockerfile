# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — build du frontend (Svelte + Vite)
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
# La sortie va dans backend/internal/api/web (voir vite.config.js)
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2 — build du backend (Go), frontend embarqué
# ---------------------------------------------------------------------------
FROM golang:1.22-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
# On récupère le frontend buildé depuis le stage précédent.
COPY --from=frontend /src/backend/internal/api/dist ./internal/api/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dcsmm ./cmd/dcsmm

# ---------------------------------------------------------------------------
# Stage 3 — image finale minimale
# ---------------------------------------------------------------------------
FROM alpine:3.20
RUN adduser -D -u 10001 dcsmm && mkdir -p /data && chown dcsmm /data
USER dcsmm
WORKDIR /app
COPY --from=backend /out/dcsmm /app/dcsmm

ENV DCSMM_HTTP_ADDR=0.0.0.0:8080 \
    DCSMM_UDP_ADDR=0.0.0.0:7778 \
    DCSMM_TCP_ADDR=0.0.0.0:7779 \
    DCSMM_DB_PATH=/data/dcsmm.db

EXPOSE 8080/tcp 7778/udp 7779/tcp
VOLUME ["/data"]

ENTRYPOINT ["/app/dcsmm"]
