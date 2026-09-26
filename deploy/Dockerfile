# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — build du frontend (Svelte + Vite)
# ---------------------------------------------------------------------------
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
# La sortie va dans backend/internal/api/dist (voir vite.config.js)
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2 — build du backend (Go), frontend embarqué
# ---------------------------------------------------------------------------
# La version de Go doit suivre backend/go.mod (Go 1.25 aujourd'hui) : un
# toolchain plus ancien refuse de compiler le module.
FROM golang:1.25-alpine AS backend
WORKDIR /src/backend
# go.sum est nécessaire : toutes les dépendances (SQLite pur Go) sont verrouillées.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Le frontend buildé vient du stage précédent : il est embarqué par //go:embed.
COPY --from=frontend /src/backend/internal/api/dist ./internal/api/dist

ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/dcsmm ./cmd/dcsmm

# ---------------------------------------------------------------------------
# Stage 3 — image finale minimale
# ---------------------------------------------------------------------------
FROM alpine:3.20
# ca-certificates : utile si un fond de carte HTTPS est utilisé côté serveur.
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 dcsmm \
 && mkdir -p /data /app \
 && chown dcsmm /data

WORKDIR /app
COPY --from=backend /out/dcsmm /app/dcsmm

# Le conteneur ne contient QUE le manager. DCS World tourne sur Windows et
# communique par le réseau (voir docs/deployment.md).
ENV DCSMM_HTTP_ADDR=0.0.0.0:8080 \
    DCSMM_UDP_ADDR=0.0.0.0:7778 \
    DCSMM_TCP_ADDR=0.0.0.0:7779 \
    DCSMM_DB_PATH=/data/dcsmm.db \
    DCSMM_TILES_DIR=/app/tiles

EXPOSE 8080/tcp 7778/udp 7779/tcp
VOLUME ["/data"]

USER dcsmm
ENTRYPOINT ["/app/dcsmm"]
