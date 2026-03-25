# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Hub + Agent binaries
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app

# Copy proto module (shared dependency)
COPY proto/ proto/

# Build Hub (needs CGO for SQLite)
COPY hub/ hub/
COPY --from=frontend /app/web/dist hub/web/dist
RUN cd hub && CGO_ENABLED=1 go build -o /out/aerodocs ./cmd/aerodocs/

# Build Agent (pure Go, cross-compile for linux/amd64 and linux/arm64)
COPY agent/ agent/
RUN cd agent && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/aerodocs-agent-linux-amd64 ./cmd/aerodocs-agent/
RUN cd agent && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/aerodocs-agent-linux-arm64 ./cmd/aerodocs-agent/

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=backend /out/aerodocs /app/aerodocs
COPY --from=backend /out/aerodocs-agent-linux-amd64 /app/aerodocs-agent-linux-amd64
COPY --from=backend /out/aerodocs-agent-linux-arm64 /app/aerodocs-agent-linux-arm64
COPY hub/static/install.sh /app/static/install.sh

VOLUME /data
EXPOSE 8081 9090

ENTRYPOINT ["/app/aerodocs"]
CMD ["--addr", ":8081", "--grpc-addr", ":9090", "--db", "/data/aerodocs.db", "--agent-bin-dir", "/app"]
