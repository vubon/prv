# Architecture & Engine Design

`prv` is a lightweight Go CLI and context extraction engine designed to prepare pull request review bundles for AI-assisted code reviews with zero extra LLM API costs.

## Component Overview

```
prv/
├── cmd/prv/
│   └── main.go             # CLI entry point and flag parsing
└── internal/
    ├── git/
    │   ├── git.go          # Git command runner (branch, diff, rev-parse)
    │   ├── github.go       # GitHub CLI adapter (`gh pr view`)
    │   └── filter.go       # Noise filter (locks, minified code, binary files)
    ├── context/
    │   ├── bundler.go      # Aggregates PR metadata + diffs into review bundle
    │   └── analyzer.go     # Detects changed symbols and impacted packages
    ├── matrix/
    │   ├── dimensions.go   # 6 review dimensions definition
    │   └── checklist.go    # Markdown checklist builder
    ├── models/
    │   ├── pr.go           # Data models for PR metadata, files, and diffs
    │   └── review.go       # Review result, issues, suggestions, comments
    └── output/
        ├── terminal.go     # Console rendering with colors and tables
        ├── markdown.go     # Structured Markdown review report generator
        └── clipboard.go    # Direct copy of review comments to OS clipboard
```

## Data Flow Pipeline

1. **Extraction**: `internal/git` fetches the base branch, PR metadata, and unified diff using local Git or the GitHub CLI (`gh`).
2. **Filtering**: `internal/git/filter.go` eliminates noisy files (e.g. `package-lock.json`, `go.sum`, generated protocol buffers).
3. **Packaging**: `internal/context/bundler.go` structures the clean diffs and metadata into a standardized review bundle.
4. **Review**: The AI Agent or reviewer analyzes the bundle against the 6 review matrix dimensions.
5. **Output**: `internal/output` formats the findings into terminal summaries, structured markdown reports, or clipboard-ready GitHub suggestion blocks.
