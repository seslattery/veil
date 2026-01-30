# Build veil binary
build:
    go build -o bin/veil ./cmd/veil

# Run all tests
test:
    go test -race ./...

# Run linter
lint:
    golangci-lint run

# Format code
fmt:
    go fmt ./...
    gofmt -s -w .

# Tidy dependencies
tidy:
    go mod tidy

# Development workflow
dev: fmt tidy lint test build
