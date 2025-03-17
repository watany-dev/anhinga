# Anhinga Project Guidelines

## Build Commands
- Build: `make build`
- Run: `make run`
- Install: `make install`
- Clean: `rm -f anhinga && go clean`

## Test Commands
- Run all tests: `go test -v ./...`
- Run specific test: `go test -v ./path/to/package -run TestName`
- Run integration tests (mohua): `cd mohua && make test-integ`
- Test coverage: `cd mohua && make cover`

## Lint Command
- Lint: `cd mohua && make lint` (requires golangci-lint)
- Install linter: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

## Code Style Guidelines
- Use Go standard formatting (`go fmt`)
- Group imports: stdlib first, then external, then internal packages
- Error handling: use wrapped errors with context (`fmt.Errorf("context: %w", err)`)
- Variable naming: camelCase with clear descriptive names
- Use interfaces for testability
- Defensive programming: check for nil pointers
- Functions should be focused with descriptive comments
- Use retries with exponential backoff for AWS API calls