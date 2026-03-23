.PHONY: dev-hub dev-web build test clean

# Development
dev-hub:
	cd hub && go run ./cmd/aerodocs/ --dev

dev-web:
	cd web && npm run dev

# Build production binary (frontend embedded in Go)
build: build-web build-hub

build-web:
	cd web && npm run build

build-hub: build-web
	cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/

# Test
test: test-hub test-web

test-hub:
	cd hub && go test ./...

test-web:
	cd web && npm test

clean:
	rm -rf bin/ web/dist/ hub/aerodocs
