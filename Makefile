.PHONY: dev-hub dev-web build test clean proto

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       proto/aerodocs/v1/agent.proto

# Development (run these in separate terminals)
dev-hub:
	cd hub && go run ./cmd/aerodocs/ --dev --addr :8080

dev-web:
	cd web && npm run dev

# Build production binary (frontend embedded)
build: build-web embed-web build-hub

build-web:
	cd web && npm run build

embed-web:
	rm -rf hub/web/dist
	mkdir -p hub/web
	cp -r web/dist hub/web/dist

build-hub:
	cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/

# Test
test: test-hub

test-hub:
	cd hub && go test ./...

clean:
	rm -rf bin/ web/dist/ hub/web/dist/ hub/aerodocs
