# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.4.0-alpha] — 2026-10-10

### Added
- **Auto-Generated Mermaid Architecture Diagrams (`internal/diagram`)**:
  - AST-powered Go import graph analysis using `go/parser` and `go/token`.
  - Discovers declared module root from `go.mod` and maps relationships across modified package boundaries and downstream internal dependencies.
  - Generates deterministic `graph LR` Mermaid syntax with touched packages accented via `classDef touched`.
  - Fallback scanner parsing unified diff additions for zero-clone / remote PR evaluations.
  - Polyglot fallback patterns for TypeScript/JavaScript and Python.
  - Native embedding in `prv bundle` context markdown and `prv report` review deliverables.
- **Dedicated `prv diagram` Standalone CLI Command**:
  - `prv diagram`: Generates diagram for current branch against main.
  - `prv diagram --staged` / `-s`: Generates diagram for uncommitted staged changes.
  - `prv diagram -b <branch>`: Generates diagram for a specific feature branch.
  - `prv diagram --markdown` / `-m`: Wraps output in ```mermaid markdown code fence.
  - `prv diagram -o <file>`: Direct file export (e.g. `architecture.mmd`).
- **Comprehensive Unit Testing**:
  - Added test suites in `internal/diagram/mermaid_test.go` verifying ID sanitization, deterministic edge rendering, Go AST module parsing, and diff fallback.

---

## [0.3.0-alpha] — 2026-10-10

### Added
- **Pre-PR Local Self-Review (`prv bundle --staged` / `-s`)**:
  - Support bundling uncommitted git staged changes (`git diff --cached`) for private pre-commit reviews.
  - Actionable terminal diagnostic if run with empty index: prompts developers to stage changes with `git add`.
  - Automatic detection of security-sensitive paths in staged index (`🚨 [SENSITIVE]`).
  - Active `.prv.yaml` team rules and 9-Dimension matrix injection into pre-commit review bundles.
  - Comprehensive unit test suite covering staged diff extraction and isolated Git environments.
- **AI Agent & Workflow Guides**:
  - Added pre-commit review guidance and prompts to Cursor IDE (`docs/agents/cursor.md`).
  - Added staged review commands to Claude Code & Claude Desktop (`docs/agents/claude.md`).
  - Added Unix pipe workflows and Git pre-commit hook script (`docs/agents/cli.md`).
  - Updated universal AI agent workflow hub (`docs/agents/README.md`) and Google Antigravity skill (`.agents/skills/prv/SKILL.md`).

---

## [0.2.0-alpha] — 2026-10-10

### Added
- **Repository Configuration (`.prv.yaml`)**:
  - Structured schema supporting `review.persona`, `review.strictness`, `review.sensitive_paths`, `review.custom_rules`, and `review.ignore_patterns`.
  - Recursive double-star glob matching (`/**` and `**/`) for security boundaries.
  - Built-in starter generator with example configuration (`.prv.yaml.example`).
  - Graceful zero-config fallback to safe defaults when `.prv.yaml` is absent.
- **CLI Commands & Flags**:
  - `prv init [path]`: Generates a fully-commented `.prv.yaml` starter file.
  - `prv bundle --config / -c`: Allows targeting custom configuration file paths.
- **Context Bundling Enhancements**:
  - Automatic `🚨 [SENSITIVE]` security path detection and `> [!CAUTION]` markdown alerts.
  - Team engineering standards injected directly into review context checklist.
- **Makefile Quality Gate**:
  - Added `make fmt`, `make fmt-check`, `make lint` (with `golangci-lint`), `make test-coverage`, `make coverage-html`, and unified `make check`.
- **AI Agent Integration Guides**:
  - Dedicated setup guides in `docs/agents/` for Google Antigravity, Cursor IDE, Claude Code / Desktop, and Headless CLI pipelines.

### Changed
- Unified glob matching across config and git noise filters; strengthened multi-segment double-star matching for nested directories.
- Safeguarded zero-clone remote PR bundling to apply local `.prv.yaml` only when running within matching repository clones.
- Standardized string builder formatting to idiomatic `fmt.Fprintf` across internal output packages.
- Handled deferred HTTP response body closing in GitHub client.

---

## [0.1.0] — 2026-10-10

### Initial Release
- Zero-clone GitHub PR bundling (`--pr`) and local branch bundling (`--branch`).
- 9-Dimension Engineering PR Review Evaluation Matrix.
- Mandatory Staff Engineer advisory deliverables: 🎯 Weakest Link in the Change & ⚖️ Architectural Trade-offs.
- Conventional Comments formatting and markdown report generation (`prv report`).
- Direct clipboard copying via `prv report --copy`.
