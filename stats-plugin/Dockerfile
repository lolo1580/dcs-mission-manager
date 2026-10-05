# Build the statistics plugin from the repository root:
#   docker build -f stats-plugin/Dockerfile -t dcsmanager-stats stats-plugin
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/stats-web .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/stats-web /stats-web
EXPOSE 8090
USER nonroot:nonroot
ENTRYPOINT ["/stats-web"]
