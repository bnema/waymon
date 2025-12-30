.PHONY: all build clean test test-integration test-capture lint fmt

# Build variables
BINARY_NAME=waymon
BUILD_DIR=dist
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Default target
all: build

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf ${BUILD_DIR}
	@go clean

# Run all tests (excluding legacy integration tests)
test:
	@echo "Running unit tests..."
	go test -v ./internal/... ./cmd/...

# Run CI-friendly integration tests (Tier 1: IPC, SSH, UseCase helpers)
test-integration:
	@echo "Running integration tests (Tier 1)..."
	go test -tags=integration -v ./tests/integration/... \
		-run 'TestIPC|TestSSH|TestServer'

# Run integration tests with race detection
test-integration-race:
	@echo "Running integration tests with race detection..."
	go test -tags=integration -race -v ./tests/integration/... \
		-run 'TestIPC|TestSSH|TestServer'

# Run only IPC integration tests
test-ipc:
	@echo "Running IPC integration tests..."
	go test -tags=integration -v ./tests/integration/... -run TestIPC

# Run only SSH integration tests
test-ssh:
	@echo "Running SSH integration tests..."
	go test -tags=integration -v ./tests/integration/... -run TestSSH

# Run Wayland-dependent tests (Tier 2, requires WAYLAND_DISPLAY)
test-integration-wayland:
	@echo "Running Wayland integration tests..."
	@if [ -z "$$WAYLAND_DISPLAY" ]; then \
		echo "Error: WAYLAND_DISPLAY not set"; \
		exit 1; \
	fi
	go test -tags=integration -v ./tests/integration/... \
		-run 'TestDisplay|TestInjection'

# Run hardware-dependent tests (Tier 3, requires root + devices)
test-integration-hardware:
	@echo "Running hardware integration tests..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: Requires root privileges"; \
		exit 1; \
	fi
	go test -tags=integration -v ./tests/integration/... \
		-run 'TestEvdev'

# Run end-to-end tests (Tier 4, requires full environment)
test-integration-e2e:
	@echo "Running end-to-end integration tests..."
	go test -tags=integration -v ./tests/integration/... \
		-run 'TestE2E'

# Run all integration tests (requires root + devices + Wayland)
test-integration-all:
	@echo "Running all integration tests..."
	go test -tags=integration -v ./tests/integration/...

# Lint the code
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Format the code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Run the server
run-server:
	@echo "Starting Waymon server..."
	${BUILD_DIR}/${BINARY_NAME} server

# Run the client
run-client:
	@echo "Starting Waymon client..."
	@read -p "Enter server address (e.g., 192.168.1.100:52525): " SERVER; \
	${BUILD_DIR}/${BINARY_NAME} client --host $$SERVER

# Development build (with race detector)
dev-build:
	@echo "Building ${BINARY_NAME} with race detector..."
	@mkdir -p ${BUILD_DIR}
	go build -race ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} .

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		internal/adapters/out/ssh/proto/events.proto

# Generate mocks
mocks:
	@echo "Generating mocks..."
	mockery

# Quick test - run basic non-interactive capture test
quick-test:
	@echo "Running quick capture test..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Note: Running without root - some tests may be skipped"; \
	fi
	go test -v -short ./internal/adapters/out/evdev/... ./internal/adapters/out/input/...

# Help
help:
	@echo "Waymon Makefile targets:"
	@echo ""
	@echo "Build:"
	@echo "  make build                 - Build the binary"
	@echo "  make dev-build             - Build with race detector"
	@echo "  make clean                 - Clean build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  make test                  - Run unit tests"
	@echo "  make test-integration      - Run CI-friendly integration tests (Tier 1)"
	@echo "  make test-integration-race - Run integration tests with race detection"
	@echo "  make test-ipc              - Run IPC tests only"
	@echo "  make test-ssh              - Run SSH tests only"
	@echo "  make test-integration-wayland  - Run Wayland tests (requires WAYLAND_DISPLAY)"
	@echo "  make test-integration-hardware - Run hardware tests (requires root)"
	@echo "  make test-integration-all  - Run all integration tests"
	@echo "  make quick-test            - Run quick adapter tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint                  - Run linter"
	@echo "  make fmt                   - Format code"
	@echo ""
	@echo "Code Generation:"
	@echo "  make proto                 - Generate protobuf files"
	@echo "  make mocks                 - Generate mock implementations"
	@echo ""
	@echo "Runtime:"
	@echo "  make run-server            - Run the server"
	@echo "  make run-client            - Run the client"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps                  - Install dependencies"