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

### 4. Save Context Bundle to File
Directly save the markdown or JSON bundle to a destination file without flooding stdout:

```bash
# Save to file using --output, --out, or -o
prv bundle --branch feat/user-auth --out bundle.md
prv bundle --pr 1 --output pr-1-bundle.md
```

### 5. Format & Save Review Report
Format a review result JSON and generate a structured markdown report:

```bash
prv report --file review-result.json
# Or using shorthand -f and save directory
prv report -f review-result.json -d reviews/
```

## Review Flags Reference

### `bundle` Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--branch` | `-b` | string | `""` | Target feature branch to review |
| `--base` | | string | `"main"` | Base branch to compare against |
| `--pr` | | string | `""` | GitHub PR number or URL (zero-clone) |
| `--repo` | `-r` | string | `"."` | Path to local Git repository |
| `--output` | `--out`, `-o` | string | `""` | Save bundle to specific file path |
| `--json` | | bool | `false` | Output bundle as raw JSON |
| `--include-locks`| | bool | `false` | Include lockfiles and generated files in diff |

### `report` Flags
| Flag | Shorthand | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--file` | `-f` | string | `""` | Input JSON file for `report` command |
| `--dir` | `-d` | string | `"reviews"`| Output directory for markdown reports |
| `--copy` | `-c` | bool | `false` | Copy formatted comments to clipboard |

