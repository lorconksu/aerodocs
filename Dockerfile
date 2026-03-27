# Build args allow CI to override private DHI base images with public equivalents
ARG NODE_IMAGE=dhi.io/node:25-debian13-dev
ARG GO_IMAGE=dhi.io/golang:1-debian13-dev

# Stage 1: Build frontend (DHI hardened Node)
FROM ${NODE_IMAGE} AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Hub + Agent binaries (DHI hardened Go)
FROM ${GO_IMAGE} AS backend
# Ensure gcc and libc6-dev are present for CGO/SQLite (no-op if already installed in DHI image)
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/* || true
WORKDIR /app

# Copy proto module (shared dependency)
COPY proto/ proto/

# Build Hub (needs CGO for SQLite)
COPY hub/ hub/
COPY --from=frontend /app/web/dist hub/web/dist
RUN cd hub && CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/aerodocs ./cmd/aerodocs/

# Build Agent (pure Go, cross-compile for linux/amd64 and linux/arm64)
COPY agent/ agent/
RUN cd agent && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/aerodocs-agent-linux-amd64 ./cmd/aerodocs-agent/
RUN cd agent && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /out/aerodocs-agent-linux-arm64 ./cmd/aerodocs-agent/

# Stage 3: Minimal runtime (Wolfi — glibc-based, fast CVE patching, no systemd/ncurses/tar)
FROM cgr.dev/chainguard/wolfi-base:latest
RUN apk add --no-cache ca-certificates-bundle tzdata && \
    adduser -D -s /bin/false aerodocs && \
    mkdir -p /data && chown aerodocs:aerodocs /data

WORKDIR /app

COPY --from=backend /out/aerodocs /app/aerodocs
COPY --from=backend /out/aerodocs-agent-linux-amd64 /app/aerodocs-agent-linux-amd64
COPY --from=backend /out/aerodocs-agent-linux-arm64 /app/aerodocs-agent-linux-arm64
COPY hub/static/install.sh /app/static/install.sh

RUN chown -R aerodocs:aerodocs /app
USER aerodocs

VOLUME /data
EXPOSE 8081 9090

ENTRYPOINT ["/app/aerodocs"]
CMD ["--addr", ":8081", "--grpc-addr", ":9090", "--db", "/data/aerodocs.db", "--agent-bin-dir", "/app"]
