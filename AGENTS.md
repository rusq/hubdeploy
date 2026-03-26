# Repository Guidelines

## Project Structure & Module Organization
`main.go` is the entrypoint for the `hubdeploy` daemon. Runtime configuration lives in `hubdeploy.yml`. Core application code is under `internal/`:

- `internal/deploysrv/` contains server setup, config loading, handlers, and logging.
- `internal/hookers/` contains webhook-specific integrations such as Docker Hub.

Tests are colocated with the code they exercise, for example `internal/deploysrv/handlers_test.go` and `internal/hookers/dockerhub_test.go`. CI workflows live in `.github/workflows/`.

## Build, Test, and Development Commands
- `go build ./...` builds all packages and matches the main CI build step.
- `go test ./...` runs the full test suite across the module.
- `go test -v ./...` shows subtest names and is the exact form used in GitHub Actions.
- `go run . -c hubdeploy.yml -v` starts the service with the local config file and verbose logging.

Use Go 1.22 to match [`go.mod`](/Users/rustam/wp/_github/hubdeploy/go.mod).

## Coding Style & Naming Conventions
Follow standard Go formatting and keep code `gofmt`-clean. Use tabs for indentation as produced by `gofmt`, lowercase package names, and mixedCaps for exported and unexported identifiers. Keep package internals in `internal/`, and prefer small focused files grouped by concern such as `config.go`, `handlers.go`, and `log.go`.

When adding flags, config fields, or webhook types, keep naming consistent with existing code and YAML keys.

## Testing Guidelines
Write table-driven tests where practical; the existing suite uses that style heavily. Keep test files next to implementation and name tests with the Go convention `TestXxx`. Use `github.com/stretchr/testify/assert` only where it improves readability; plain `testing` checks are also common here.

Run `go test ./...` before opening a PR. Add or update tests for any change to request handling, config parsing, or webhook registration.

## Commit & Pull Request Guidelines
Recent commits use short, imperative subjects such as `update deps`, `code cleanup`, and `run as a daemon`. Keep commit titles concise and descriptive.

PRs should include a brief summary, the reason for the change, and test results from `go test ./...`. Link related issues when applicable. Include config or API examples when behavior changes affect webhook payloads or server startup.
