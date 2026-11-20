# Copilot Instructions for plakar

These instructions guide code generation and edits in this repository.

## Scope and language
- Primary language: Go.
- Keep changes small, focused, and idiomatic.
- Prefer standard library first; introduce new dependencies only when clearly justified.

## Coding standards
- Follow idiomatic Go formatting and style (`gofmt` output, clear names, early returns on errors).
- Preserve existing architecture and package boundaries (`api/`, `subcommands/`, `utils/`, `appcontext/`, etc.).
- Do not perform broad refactors unless explicitly requested.
- Keep exported APIs and CLI behavior stable unless the task asks for a behavior change.
- For platform-specific behavior, keep split files by OS when appropriate (`*_windows.go`, `*_unix.go`).

## Tooling and validation
- Use Go version from `go.mod`.
- Core build command:
  - `go build -v ./...`
- Preferred local validation sequence before proposing completion:
  - `go test ./...`
  - `make coverage` when coverage-sensitive changes are made.
- If touching man pages (`*.[1-9]`), ensure they remain `mandoc -Tlint -Wstyle -l` clean and synced with `.goreleaser.yml`.

## Testing conventions
- Add or update tests for every behavior change.
- Prefer table-driven tests where multiple input/output cases exist.
- Existing test style heavily uses `github.com/stretchr/testify/require`; keep consistency with nearby tests.
- Keep tests deterministic and isolated:
  - Use `t.TempDir()` for filesystem state.
  - Avoid network/external dependencies unless already mocked/faked in the package.
- Keep assertions precise (error type/message where relevant).

## Coverage expectations
- CI runs tests with coverage profile and atomic mode.
- Coverage profile file: `coverage.out`.
- Local coverage command:
  - `make coverage`
- Coverage intentionally excludes helper packages under `testing/` from totals (profile filtering mirrors Codecov ignore settings).
- For changes in critical paths (`api/`, `subcommands/`, `utils/`, `config/`, `cached/`), include targeted tests to avoid coverage regressions.

## CI alignment
- Linux CI validates:
  - `go build -v ./...`
  - `go test -v -coverprofile=coverage.out -covermode=atomic -timeout 1m -json ./...`
  - Uploads coverage and JUnit test reports to Codecov.
- Windows CI currently builds the project; keep cross-platform compilation in mind.

## Change hygiene for Copilot
- Update docs when changing user-facing CLI behavior or flags.
- Preserve existing error handling semantics and exit code behavior.
- Prefer edits that are straightforward to review: minimal diff, clear rationale, and matching tests.
