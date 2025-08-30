.PHONY: fmt lint fix check install-tools dev build run clean test

# Variables
BINARY_NAME := server
BUILD_DIR := ./bin
PORT := 8080

# Development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/cosmtrek/air@latest  # Live reload
	@echo "✓ Tools installed"

# Code quality
fmt:
	@echo "Formatting code..."
	gofumpt -w .
	goimports -w .
	@echo "✓ Code formatted"

lint:
	golangci-lint run

fix: fmt
	@echo "Auto-fixing issues..."
	golangci-lint run --fix
	@echo "✓ Issues fixed"

check: fix lint
	@echo "✓ Code quality check complete"

# Build and run
build: check
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Development with live reload
dev: check
	air -c .air.toml

# Testing
test: check
	go test -v ./...

test-coverage: check
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Clean up
clean:
	go clean
	rm -rf $(BUILD_DIR)
	rm -f coverage.out