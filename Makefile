.PHONY: build test test-v test-integration lint clean

# Build the CLI binary
build:
	go build -o bin/agent-fleet ./cmd/agent-fleet

# Run all tests (unit + integration, no Docker)
test:
	go test -timeout 30s ./...

# Run tests with verbose output
test-v:
	go test -timeout 30s -v ./...

# Run E2E tests (requires Docker)
test-e2e:
	AGENT_FLEET_E2E=1 go test -tags integration -timeout 120s ./tests/integration/

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/ .agent-fleet/
