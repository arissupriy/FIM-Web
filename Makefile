# Makefile for OJS Monitor (FIM Platform)

.PHONY: build clean test test-race status migrate help

# Default target
help:
	@echo "OJS Monitor - FIM Platform"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Build all binaries (manage, fim-server, worker)"
	@echo "  make clean         Remove binaries"
	@echo "  make test          Run tests"
	@echo "  make test-race     Run tests with race detector"
	@echo "  make status        Show system status"
	@echo "  make migrate       Run database migrations"
	@echo "  make server        Start HTTP server"
	@echo "  make worker        Start background worker"
	@echo "  make help          Show this help"

# Build all binaries
build:
	@echo "Building binaries..."
	cd backend && go build -o manage ./cmd/manage
	cd backend && go build -o fim-server ./cmd/server
	cd backend && go build -o worker ./cmd/worker
	@echo "✓ Build complete"
	@echo "  - manage"
	@echo "  - fim-server"
	@echo "  - worker"

# Build individual binaries
manage:
	cd backend && go build -o manage ./cmd/manage

fim-server:
	cd backend && go build -o fim-server ./cmd/server

worker:
	cd backend && go build -o worker ./cmd/worker

# Clean binaries
clean:
	rm -f backend/manage backend/fim-server backend/worker
	@echo "✓ Binaries removed"

# Run tests
test:
	cd backend && go test ./...

# Run tests with race detector
test-race:
	cd backend && go test -race ./...

# Show system status
status:
	cd backend && ./manage status

# Run migrations
migrate:
	cd backend && ./manage migrate

# Start HTTP server (requires worker in separate terminal)
server:
	cd backend && ./fim-server

# Start background worker
run-worker:
	cd backend && ./worker

# Development targets
dev: build
	@echo ""
	@echo "Setup complete! Next steps:"
	@echo "  1. ./manage migrate    - Run migrations"
	@echo "  2. ./manage seed      - Create default admin"
	@echo "  3. ./fim-server      - Start API server (terminal 1)"
	@echo "  4. ./worker          - Start worker (terminal 2)"
