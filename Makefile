# Makefile for OJS Monitor (FIM Platform)

.PHONY: build clean test test-race status migrate help dev
.PHONY: server-start server-stop server-restart server-status

# Binary output directory
BIN_DIR := bin

# Build targets
BUILD_MANAGE := $(BIN_DIR)/manage
BUILD_SERVER := $(BIN_DIR)/fim-server
BUILD_WORKER := $(BIN_DIR)/worker

# Default target
help:
	@echo "OJS Monitor - FIM Platform"
	@echo ""
	@echo "Usage:"
	@echo "  make build              Build all binaries"
	@echo "  make clean            Remove binaries"
	@echo "  make test             Run tests"
	@echo "  make test-race        Run tests with race detector"
	@echo ""
	@echo "  make status           Show system status"
	@echo "  make migrate          Run database migrations"
	@echo ""
	@echo "  make server-start     Start server + worker (daemon)"
	@echo "  make server-stop     Stop server + worker"
	@echo "  make server-restart  Restart all services"
	@echo "  make server-status   Show service status"
	@echo ""
	@echo "  make help             Show this help"

# Build all binaries
build: $(BUILD_MANAGE) $(BUILD_SERVER) $(BUILD_WORKER)
	@echo "✓ Build complete"
	@echo "  Binaries in: $(BIN_DIR)/"
	@echo "  - manage"
	@echo "  - fim-server"
	@echo "  - worker"

$(BUILD_MANAGE):
	@echo "  Building manage..."
	@mkdir -p $(BIN_DIR)
	cd backend && go build -o ../$(BIN_DIR)/manage ./cmd/manage

$(BUILD_SERVER):
	@echo "  Building fim-server..."
	@mkdir -p $(BIN_DIR)
	cd backend && go build -o ../$(BIN_DIR)/fim-server ./cmd/server

$(BUILD_WORKER):
	@echo "  Building worker..."
	@mkdir -p $(BIN_DIR)
	cd backend && go build -o ../$(BIN_DIR)/worker ./cmd/worker

# Clean binaries
clean:
	rm -rf $(BIN_DIR)
	@echo "✓ Binaries removed"

# Run tests
test:
	cd backend && go test ./...

# Run tests with race detector
test-race:
	cd backend && go test -race ./...

# Show system status
status:
	cd backend && ./$(BIN_DIR)/manage status

# Run migrations
migrate:
	cd backend && ./$(BIN_DIR)/manage migrate

# Server management (via manage CLI)
server-start:
	cd backend && ./$(BIN_DIR)/manage server:start

server-stop:
	cd backend && ./$(BIN_DIR)/manage server:stop

server-restart:
	cd backend && ./$(BIN_DIR)/manage server:restart

server-status:
	cd backend && ./$(BIN_DIR)/manage server:status

# Development targets
dev: build migrate
	@echo ""
	@echo "✓ Setup complete!"
	@echo ""
	@echo "Quick Start:"
	@echo "  make server-start   # Start server + worker"
	@echo "  make server-stop    # Stop services"
	@echo "  make server-status  # Check status"
	@echo ""
	@echo "CLI Commands:"
	@echo "  ./bin/manage migrate                    # Run migrations"
	@echo "  ./bin/manage seed                     # Seed default admin"
	@echo "  ./bin/manage add-admin <user> <pass>  # Create admin"
	@echo "  ./bin/manage server:start             # Start services"
	@echo "  ./bin/manage server:stop              # Stop services"
	@echo "  ./bin/manage server:status            # Show status"
