# CLI Usage & Commands

## Installation

```bash
# Build binary locally
make build

# Or install into $GOPATH/bin
make install
```

## Commands

### 1. Bundle Context for a Local Branch
Extract unified diffs and PR metadata for the current or specified local branch:

```bash
# Current or target branch against default base branch (main/master)
prv bundle --branch feat/user-auth

# Target branch against a custom base branch
prv bundle --branch feat/user-auth --base develop

# Output review bundle as JSON
prv bundle --branch feat/user-auth --json
```

### 2. Zero-Clone Remote PR Review
Review any GitHub PR without cloning the repository or checking out branches locally:

```bash
# Using PR number (inside a git repository with GitHub remote)
prv bundle --pr 42

# Using full GitHub PR URL
prv bundle --pr https://github.com/org/repo/pull/42
```

### 3. Review a Specific Local Folder
```bash
prv bundle --repo /path/to/another/repo --branch feat/payment
```

### 4. Format & Save Review Report
Format a review result JSON and generate a structured markdown report:

```bash
prv report --file review-result.json
```

## Review Flags Reference

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--branch` | string | `""` | Target feature branch to review |
| `--base` | string | `"main"` | Base branch to compare against |
| `--pr` | string | `""` | GitHub PR number or URL (zero-clone) |
| `--repo` | string | `"."` | Path to local Git repository |
| `--json` | bool | `false` | Output bundle in JSON format |
| `--file` | string | `""` | Input JSON file for `report` command |
