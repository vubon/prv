# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.9.0-alpha] — 2026-10-15

### Added
- **Flexible Base Branch Comparison (`--base`, `-B`)**:
  - Review the current branch against any integration branch (e.g., `staging`, `dev`, `develop`) without needing to explicitly supply `--branch`:
    `prv bundle --base staging` or `prv bundle -B dev`.
  - Added support for `--base, -B` across `prv diagram` and `prv walkthrough`.
- **Positional Branch & PR Argument Resolution**:
  - Compare current branch against base with positional shorthand: `prv bundle staging` or `prv bundle dev`.
  - Compare explicit feature branch against base with two positional arguments: `prv bundle feat/auth staging`.
  - Automatically identifies common base branches (`staging`, `dev`, `develop`, `main`, `master`, `next`, `trunk`).
- **Configurable `default_base_branch` in `.prv.yaml`**:
  - Repositories using non-main trunk or GitFlow branches can configure `review.default_base_branch: "staging"`.
  - Defaults to repo config before falling back to automatic git detection.
- **Enhanced Git Ref Resolution & Local Commit Preservation (`internal/git/git.go`)**:
  - Prioritizes local branch commits on `headRef` rather than prematurely overriding with `origin/<head>`, preserving unpushed local commits.
  - Automatically attempts `git fetch origin <base>` when remote base branches haven't been fetched locally.
  - Expanded `GetDefaultBaseBranch()` fallback order across `main`, `master`, `develop`, `dev`, and `staging`.

---

## [0.8.0-alpha] — 2026-10-12

### Added
- **Production-Grade GitHub Actions CI/CD Pipeline (`.github/workflows/ci.yml`)**:
  - Triggers on Pull Requests to `main` and pushes/merges to `main`.
  - Enforces code formatting via `gofmt` and runs static analysis with `go vet` and `golangci-lint`.
  - Multi-OS test matrix across **Ubuntu**, **macOS**, and **Windows** on Go 1.26.x and 1.27.x with Go race detector (`-race`).
  - Code coverage reporting with `coverage.out` artifact uploads.
  - Quick compilation and smoke test step (`--version`, `--help`).
- **Automated Cross-Platform Release Workflow (`.github/workflows/release.yml`)**:
  - Automatically compiles stripped static binaries (`CGO_ENABLED=0`) across 5 platforms (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64`).
  - Packages `.tar.gz` and `.zip` archives with SHA-256 `checksums.txt`.
  - Automatically publishes assets to GitHub Releases via `softprops/action-gh-release@v2`.
- **Comprehensive Modern `README.md`**:
  - Added CI build, release, Go version, Conventional Comments, and MIT license badges.
  - Clear "Why prv?" problem statement detailing diff noise reduction.
  - Interactive Mermaid architecture impact diagram demonstration.
  - 5 core workflows with copy-ready CLI commands (pre-commit reviews, diagrams, walkthroughs, review reporting).
  - Conventional Comments 8-label quick reference table with drop-in suggestion blocks.
  - Human-in-the-loop AI pairing architecture diagram and guidelines.
  - Complete `.prv.yaml` schema and CLI flag reference table.
- **Community Guidelines & Licensing**:
  - Added [`CONTRIBUTING.md`](CONTRIBUTING.md) detailing development setup, testing, pre-commit self-reviews, and Conventional Commits.
  - Added [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) adhering to Contributor Covenant v2.1.
  - Added standard MIT [`LICENSE`](LICENSE).

---

## [0.7.0-alpha] — 2026-10-11

### Added
- **GitHub Pull Request Reviews & Inline Diff Comments (`internal/git/github.go`)**:
  - Implemented `PostPRReview`, `PostPRReviewViaCLI`, and `PostPRReviewViaAPI`.
  - Added support for publishing full GitHub PR Reviews via `POST /repos/{owner}/{repo}/pulls/{number}/reviews` and `gh api`.
  - Publishes line-level inline comments pinned directly to modified code lines in GitHub's "Files changed" diff tab.
  - Added graceful 422 diff-anchoring fallback that resends reviews without inline comments if lines fall outside the active PR diff.
  - Implemented `ResolveRepoAndPR` for unified remote inference and PR target resolution.
- **Conventional Comments Formatting Standard (`internal/output/comments.go`)**:
  - Aligned `FormatConventionalComment` strictly with the Conventional Comments specification: `<label> [decorations] (<subject>): <title>`.
  - Added support for `Subject` field in `models.Issue` and `models.Suggestion` (e.g. `(nil-check)`, `(sqli)`, `(naming)`).
  - Added support for Question (`❓ question`) and Praise (`👏 praise`) labels.
  - Implemented `GenerateInlineComments` and `BuildReviewPayload` for structured review generation.
- **CLI Command Support (`prv report --inline`)**:
  - Added `--inline` / `-i` flag to `prv report`.
  - Automatically posts GitHub Pull Request Reviews with inline diff comments whenever line-level issues or suggestions exist.
- **Comprehensive Unit Testing Suite**:
  - Added tests for `ResolveRepoAndPR`, `PostPRReview` validation, and `PostPRReviewViaAPI` with mock server in `internal/git/github_test.go`.
  - Added tests for subject overrides, question/praise labels, and `GenerateInlineComments` in `internal/output/comments_test.go`.
  - Added `TestBuildReviewPayload` in `internal/output/markdown_test.go`.

---

## [0.6.0-alpha] — 2026-10-10

### Added
- **Direct GitHub PR Comment Posting (`internal/git/github.go`)**:
  - Added `PostPRComment`, `PostPRCommentViaCLI`, and `PostPRCommentViaAPI`.
  - Supports publishing review reports directly via `gh pr comment <pr> --body-file -` with stdin streaming to prevent shell ARG_MAX limitations.
  - Automatic fallback to GitHub REST API (`POST /repos/{owner}/{repo}/issues/{number}/comments`) when `GITHUB_TOKEN` is present.
  - Implemented `ParseRepoFromRemoteURL` to resolve repository owner and name from HTTPS, SSH (`git@github.com:...`), and `ssh://` remotes.
  - Added configurable `BaseURL` on `GitHubClient` for deterministic testing against mock HTTP servers.
- **CLI Command Support (`prv report --post`)**:
  - Added `--post` / `-p` and `--pr` flags to `prv report`.
  - Automatically identifies PR targets from CLI flags or review JSON `pr_number` / `pr_url`.
- **Comprehensive Unit Testing Suite**:
  - Added `internal/git/github_test.go` with table-driven tests for URL parsing, remote slug extraction, validation, and mock API servers.
  - Added `internal/output/clipboard_test.go` and `internal/output/terminal_test.go`.
  - Added `TestBundler_ToJSON` in `internal/context/bundler_test.go`.
  - All test suites passing 100% across all packages.

---

## [0.5.0-alpha] — 2026-10-10

### Added
- **Collapsible File-by-File Changes Walkthrough (`internal/output/walkthrough.go`)**:
  - Interactive `<details><summary>...</summary></details>` blocks summarizing each modified file.
  - Formats file path, addition/deletion stats (`(+N / -M)`), sensitive badges (`🚨 [SENSITIVE]`), and thematic topic labels.
  - Automatic AST and diff hunk heuristic analysis (`GenerateWalkthroughFromFiles`): extracts modified function signatures (`func Foo()`) and type definitions (`type Bar struct`), ignoring comments and non-code lines.
  - Clean GitHub Flavored Markdown compliance with newline separation between `<summary>` and markdown list elements.
  - Native embedding in `prv bundle` context markdown and `prv report` review deliverables.
- **Dedicated `prv walkthrough` Standalone CLI Command**:
  - `prv walkthrough`: Generates walkthrough for current branch against main.
  - `prv walkthrough --staged` / `-s`: Generates walkthrough for uncommitted staged changes.
  - `prv walkthrough -b <branch>`: Generates walkthrough for a specific feature branch.
  - `prv walkthrough --json`: Outputs structured walkthrough as JSON.
  - `prv walkthrough -o <file>`: Direct file export (e.g. `walkthrough.md`).
- **Comprehensive Unit Testing**:
  - Added unit test suite in `internal/output/walkthrough_test.go` verifying formatting, edge cases, sensitive paths, topic inference, and symbol extraction.
  - Added integration tests in `internal/output/markdown_test.go` and `internal/context/bundler_test.go`.

---

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
