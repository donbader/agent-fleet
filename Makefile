.PHONY: build test lint clean

# Build the CLI binary
build:
	go build -o bin/agent-fleet ./cmd/agent-fleet

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-v:
	go test -v ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/
