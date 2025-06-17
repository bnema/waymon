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

# Run all tests
test:
	@echo "Running unit tests..."
	go test -v ./...

# Run integration tests
test-integration: test-capture test-network test-wayland

# Run capture integration test
test-capture:
	@echo "Running capture integration test..."
	@echo "Note: This test requires read/write access to /dev/input devices"
	@go run tests/integration/capture/main.go -v

# Run interactive capture test
test-capture-interactive:
	@echo "Running interactive capture integration test..."
	@echo "Note: This test requires read/write access to /dev/input devices"
	@echo "Safety: Devices auto-release after 5 seconds (or press ESC)"
	@go run tests/integration/capture/main.go -v -i -d 10s

# Run network integration test
test-network:
	@echo "Running network integration test..."
	@go run tests/integration/network/main.go

# Run Wayland injection test
test-wayland:
	@echo "Running Wayland injection test..."
	@echo "Note: This test requires a running Wayland compositor"
	@go run tests/integration/wayland/main.go -v

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
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/protocol/*.proto

# Quick test - run basic non-interactive capture test
quick-test:
	@echo "Running quick capture test..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Note: Running without root - some tests may be skipped"; \
	fi
	go test -v -short ./internal/input/...

# Help
help:
	@echo "Waymon Makefile targets:"
	@echo "  make build                 - Build the binary"
	@echo "  make clean                 - Clean build artifacts"
	@echo "  make test                  - Run unit tests"
	@echo "  make test-capture          - Run basic capture integration test"
	@echo "  make test-capture-interactive - Run interactive capture test (5s timeout)"
	@echo "  make test-network          - Run network/SSH transport tests"
	@echo "  make test-wayland          - Run Wayland injection tests"
	@echo "  make lint                  - Run linter"
	@echo "  make fmt                   - Format code"
	@echo "  make deps                  - Install dependencies"
	@echo "  make run-server            - Run the server"
	@echo "  make run-client            - Run the client"
	@echo "  make dev-build             - Build with race detector"
	@echo "  make proto                 - Generate protobuf files"
	@echo "  make quick-test            - Run quick tests"