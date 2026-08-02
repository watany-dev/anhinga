# Anhinga Project Guidelines

## Build Commands
- Build: `make build`
- Run: `make run`
- Install: `make install`
- Clean: `make clean`

## Test Commands
- Run all tests: `make test`
- Run with the race detector: `make test-race`
- Test coverage: `make cover`
- Run specific test: `go test -v ./path/to/package -run TestName`

## Lint Command
- Lint: `make lint` (golangci-lint v2, config in `.golangci.yml`)
- Auto-format: `make fmt`
- Vet: `make vet`
- Install the linter: `make tools`

## CI
- `ci.yml`: build, `make test-race`, `make cover`, golangci-lint
- `security.yml`: semgrep, govulncheck, gitleaks (advisory; they report to the
  Security tab but do not fail the build yet)

## Code Style Guidelines
- Use Go standard formatting (`go fmt`)
- Group imports: stdlib first, then external, then internal packages
- Error handling: use wrapped errors with context (`fmt.Errorf("context: %w", err)`)
- Variable naming: camelCase with clear descriptive names
- Use interfaces for testability
- Defensive programming: check for nil pointers
- Functions should be focused with descriptive comments
- Use retries with exponential backoff for AWS API calls