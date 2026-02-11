# OpenTL Makefile

.PHONY: build install clean test lint sandbox-image server-image docker-up docker-down

# Build the opentl binary.
build:
	go build -o bin/opentl ./cmd/opentl

# Install the opentl binary to $GOPATH/bin.
install:
	go install ./cmd/opentl

# Run tests.
test:
	go test ./...

# Run linter.
lint:
	golangci-lint run ./...

# Build the sandbox Docker image.
sandbox-image:
	docker build -f docker/base.Dockerfile -t opentl-sandbox .

# Build the server Docker image.
server-image:
	docker build -f docker/server.Dockerfile -t opentl-server .

# Start everything with Docker Compose.
docker-up: sandbox-image
	docker compose -f docker/compose.yml up -d

# Stop everything.
docker-down:
	docker compose -f docker/compose.yml down

# Clean build artifacts.
clean:
	rm -rf bin/
	go clean
