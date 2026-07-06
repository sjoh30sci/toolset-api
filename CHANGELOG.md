# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and the project adheres to
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added (Phase 5 — Distribution & Packaging)

- Cross-platform CLI release automation via GoReleaser (`.goreleaser.yml`).
- `toolset package` command to build a portable distribution tarball.
- OpenAPI 3.0 spec generator (`gateway --openapi-spec`) driving SDK generation.
- TypeScript SDK (`sdk/ts`, `@yourusername/toolset-api`) and Python SDK
  (`sdk/py`, `toolset-api`).
- GitHub Actions: `Test & Build` and `Release` workflows.
- Documentation: `QUICKSTART`, `EMBEDDING`, `API_REFERENCE`, `TROUBLESHOOTING`.
- Project metadata: `LICENSE` (MIT), `CONTRIBUTING`, `SECURITY`, issue/PR
  templates, `CODEOWNERS`, release checklist, `INSTALL.sh`.
- Makefile targets: `openapi`, `sdk-generate`, `package`, `release`, `docs`,
  `install-dev`.

## [0.1.0] — Phases 1–4

- Phase 1: Project foundation — docker-compose, Go gateway, SQLite, auth, CLI.
- Phase 2: SearXNG + file-server services, gateway handlers, integration tests.
- Phase 3: Code executor with nsjail, async job queue, 9 language runtimes.
- Phase 4: Browser automation with Playwright, full MCP compliance.
