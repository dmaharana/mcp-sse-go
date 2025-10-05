# CRUSH.md - Development Guidelines for MCP HTTP Go

## Build, Test & Lint Commands

# Build the application
build: go build -o ./bin/mcp-server -ldflags="-s -w" ./cmd/mcp-server

# Clean build artifacts
clean: rm -rf bin/

# Run all tests
test: go test -v ./...

# Run specific package tests
test-package: go test -v ./internal/session/

# Run single test file
test-file: go test -v ./internal/session/handlers_test.go

# Run specific test
test-specific: go test -v -run TestHandlerName ./internal/session/

# Format code
format: go fmt ./...

# Vet code for issues
vet: go vet ./...

# Lint (if golangci-lint is available)
lint: golangci-lint run

## Code Style Guidelines

### Imports
- Use stdlib imports first, then third-party, then local packages
- Group imports with blank lines between groups
- Use explicit package names for clarity when needed
- Remove unused imports

### Formatting
- Use go fmt for consistent formatting
- 4-space indentation
- Line length: aim for 100 chars max (flexible for readability)
- Use goimports for import organization

### Naming Conventions
- Use camelCase for variables and functions
- Use PascalCase for exported types and functions
- Use descriptive names (avoid abbreviations)
- Prefix interface types with "er" when appropriate (e.g., SessionManager)
- Use context.Context parameter named "ctx"
- Error types end with "Error" (e.g., SessionError)

### Types
- Prefer concrete types over interfaces when possible
- Use struct tags for JSON serialization
- Define custom types for domain concepts
- Use pointer receivers for methods that modify structs
- Export types/functions that are part of public API

### Error Handling
- Always check and handle errors explicitly
- Use errors.Wrap/WithStack for error context
- Return errors, don't log and return nil
- Use structured logging with zerolog
- Add "component" field to logger for tracing

### Testing
- Write tests for all public functions
- Use table-driven tests when possible
- Include both unit and integration tests
- Test error paths, not just success cases
- Use testify/assert for assertions if needed

### Logging
- Use zerolog for structured logging
- Always include relevant context fields
- Use appropriate log levels (debug, info, warn, error)
- Never log sensitive information

### Go Version
- Target Go 1.22+
- Use generics when beneficial
- Leverage new stdlib features appropriately