.PHONY: dev-hub dev-web build test clean proto

# Development (run these in separate terminals)
dev-hub:
	cd hub && go run ./cmd/aerodocs/ --dev --addr :8080

dev-web:
	cd web && npm run dev

# Proto generation
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       proto/aerodocs/v1/agent.proto

# Build production binary (frontend embedded)
build: build-web embed-web proto build-agent build-hub

build-web:
	cd web && npm run build

embed-web:
	rm -rf hub/web/dist
	mkdir -p hub/web
	cp -r web/dist hub/web/dist

build-hub:
	cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/

build-agent:
	cd agent && GOOS=linux GOARCH=amd64 go build -o ../bin/aerodocs-agent-linux-amd64 ./cmd/aerodocs-agent/
	cd agent && GOOS=linux GOARCH=arm64 go build -o ../bin/aerodocs-agent-linux-arm64 ./cmd/aerodocs-agent/

# Test
test: test-hub test-agent

test-hub:
	cd hub && go test ./...

test-agent:
	cd agent && go test ./...

clean:
	rm -rf bin/ web/dist/ hub/web/dist/ hub/aerodocs
