# Repository Guidelines

## Project Structure & Module Organization
- Module name is `github.com/tendant/postgres-mcp-go`; keep imports aligned with it.
- Place runnable entry points under `cmd/` (e.g., `cmd/agent/main.go`) and reusable packages under `internal/` or `pkg/`.
- Store SQL fixtures, migrations, or prompt templates in `assets/`, and environment samples in `configs/`.
- Use `testdata/` for deterministic fixtures; it is automatically ignored by Go tools.
- Keep generated binaries, coverage files, and local `.env` secrets out of version control (already covered in `.gitignore`).

## Build, Test, and Development Commands
- `go mod tidy` — synchronize `go.mod` after adding or removing imports.
- `go fmt ./...` — apply canonical Go formatting before every commit.
- `go build ./...` — verify all packages compile with the current toolchain.
- `go test ./...` — run the full unit test suite; add `-race` when touching concurrency.
- `go run ./cmd/agent` — execute the default agent entry point once it exists.

## Coding Style & Naming Conventions
- Follow Go 1.24 defaults: tabs for indentation, one statement per line, no unnecessary semicolons.
- Use concise, lowercase package names (`postgres`, `agent`) and PascalCase for exported types and functions.
- Prefix sentinel errors with `Err` and wrap upstream errors with `%w` to preserve context.
- Document exported identifiers with complete sentences so `godoc` output stays clear.

## Testing Guidelines
- Organize tests as table-driven cases in `*_test.go` files colocated with the code they cover.
- Prefer standard library `testing` plus `testdata/` fixtures; add integration tests under `internal/<pkg>/integration_test.go` when needed.
- Maintain at least 80% package coverage; verify with `go test -cover ./...` before opening a PR.

## Commit & Pull Request Guidelines
- Write imperative, 1-line commit subjects similar to the existing history (`add go.mod`).
- Squash unrelated work; each commit should build and pass tests.
- PR descriptions must include the problem statement, the solution outline, and local command output (`go test ./...`).
- Link issues with `Fixes #123` or `Refs #123`, and request review only after CI succeeds.

## Security & Configuration Notes
- Never commit secrets; keep runtime credentials in `.env`, and publish sample placeholders in `configs/.env.example`.
- Document required PostgreSQL roles, ports, and external service dependencies alongside new features.
