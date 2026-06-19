# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS builder

# Keep the Go toolchain's memory footprint low so the build does not OOM the
# host. A cold `go build` of this dependency tree (gin + sonic + pgx +
# validator + net/http2 + quic-go) peaks at ~1.2 GB while linking; on a small
# Dokploy VPS that also runs the panel/Traefik/Postgres that spike trips the
# OOM killer and takes the whole box down.
#
#   GOGC=20      makes the compiler/linker GC aggressively  (~1.2 GB -> ~0.8 GB peak)
#   GOMAXPROCS/-p limit how many package compiles run at once (caps the spike on multi-core hosts)
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOGC=20 \
    GOMAXPROCS=2 \
    GOFLAGS=-p=2

WORKDIR /app

# Download modules in their own cached layer so source changes don't refetch.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Persist the compile cache across builds. After the first build only the
# packages that actually changed are recompiled, so subsequent deploys drop
# from ~1.2 GB / ~40 s to a few tens of MB / a couple of seconds.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o pensieri ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/pensieri .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/web ./web

EXPOSE 8080

CMD ["./pensieri"]
