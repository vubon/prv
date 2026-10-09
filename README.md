# `prv` — PR Review Context Engine & CLI (Go)

A fast, lightweight Go CLI tool that prepares Pull Request review contexts, diffs, metadata, and noise-filtered code packages for AI-assisted code reviews.

## Commands Overview

```bash
# Bundle PR context for the current or target branch
prv bundle --branch <branch_name>

# Bundle PR context against a specific base branch
prv bundle --branch <branch_name> --base main

# Bundle PR context using GitHub PR number or URL
prv bundle --pr <pr_number_or_url>

# Format and save a review report into reviews/
prv report --file <review_output.json>
```

## Structure

- `cmd/prv/`: CLI entry point and command flags
- `internal/git/`: Git diff extraction and GitHub CLI metadata fetcher
- `internal/context/`: Noise filter (locks/binaries) and review context bundler
- `internal/matrix/`: Review matrix dimensions & checklists
- `internal/output/`: Terminal formatter, Markdown generator, and clipboard exporter

## Documentation

- [docs/usage.md](docs/usage.md): Full CLI usage guide, zero-clone ingestion, and flag reference.
- [docs/architecture.md](docs/architecture.md): Internal engine architecture and data flow pipeline.
- [docs/conventional_comments.md](docs/conventional_comments.md): Conventional Comments specification and review labels.

