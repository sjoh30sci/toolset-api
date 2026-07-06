# Contributing to Toolset API

Thanks for your interest in contributing! This document covers the workflow and
conventions for the project.

## Development Setup

```bash
git clone https://github.com/yourusername/toolset-api
cd toolset-api
make build-cli   # CLI (no C compiler needed, CGO_ENABLED=0)
make build       # gateway (needs a C compiler for go-sqlite3)
make test
```

## Project Layout

```
gateway/   Go HTTP + MCP gateway (echo, sqlite)
cli/       `toolset` CLI (cobra)
sdk/ts     TypeScript SDK
sdk/py     Python SDK
docs/      Documentation
tools/     Tool service definitions (search, files, exec, browser)
```

## Workflow

1. Fork and create a feature branch off `main`.
2. Make your change with tests.
3. Run `gofmt -w` and `make test` before committing.
4. Open a pull request against `main`. Fill out the PR template.

## Commit Messages

Use short, imperative subjects. Prefixes such as `test:`, `chore`, and `docs:`
are excluded from the release changelog (see `.goreleaser.yml`). Feature and fix
commits should be descriptive:

```
Add browser PDF export action
Fix rate-limit reset window off-by-one
```

## Code Style

- Go: idiomatic Go, `gofmt`/`go vet` clean, keep handlers thin.
- TypeScript: `tsc` strict mode.
- Python: type hints, `>= 3.8` compatible.

## Testing

- `make test` — gateway unit tests.
- `make test-e2e` — docker-compose smoke test (requires Docker).
- Add tests alongside new handlers (`*_test.go`).

## Releasing

Maintainers only — see [`.github/release-checklist.md`](.github/release-checklist.md).
