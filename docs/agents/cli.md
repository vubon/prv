# Headless Terminal & CLI Pipeline Guide

`prv` is built according to Unix philosophy: it produces clean, structured markdown or JSON to stdout, making it directly composable with terminal LLM clients, automation scripts, and Git hooks.

---

## 1. Piping into Terminal LLMs

### Using `llm` (Simon Willison's CLI)

```bash
# Review current branch with Claude 3.7 Sonnet
prv bundle --branch feat/auth | llm -m claude-3-7-sonnet \
  -s "You are a Staff Engineer reviewing this PR context. Identify bugs, the weakest link, and architectural trade-offs."

# Review remote GitHub PR with GPT-4o
prv bundle --pr 42 | llm -m gpt-4o \
  -s "Review this PR against the 9-dimension checklist and provide Conventional Comments."
```

### Using `aichat`

```bash
prv bundle --branch feat/cache | aichat -r staff-reviewer
```

### Using GitHub Copilot CLI (`gh copilot`)

```bash
prv bundle --branch feat/billing --json | gh copilot explain
```

---

## 2. Terminal Review Helper Script

You can add a simple helper function to your `~/.zshrc` or `~/.bashrc`:

```bash
# Review local branch or PR with prv + LLM
prv-review() {
  local target="${1:-$(git rev-parse --abbrev-ref HEAD)}"
  echo "🔍 Bundling review context for: $target ..."
  
  if [[ "$target" =~ ^[0-9]+$ ]] || [[ "$target" =~ ^https:// ]]; then
    prv bundle --pr "$target" | llm -m claude-3-7-sonnet
  else
    prv bundle --branch "$target" | llm -m claude-3-7-sonnet
  fi
}
```

Usage:
```bash
prv-review                  # Reviews current checked-out branch
prv-review feat/stripe      # Reviews specific branch
prv-review 42               # Reviews GitHub PR #42
```

---

## 3. Git Pre-Push Hook Integration

To run automated pre-push checks and ensure no sensitive security paths are touched without review:

Create `.git/hooks/pre-push`:

```bash
#!/usr/bin/env bash
set -e

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
  echo "🔎 [prv] Checking branch $CURRENT_BRANCH against .prv.yaml sensitive paths..."
  
  # Run prv bundle to check sensitive files and diff stats
  prv bundle --branch "$CURRENT_BRANCH" --output /tmp/prv-bundle.md
  
  if grep -q "Security Sensitive Paths Touched" /tmp/prv-bundle.md; then
    echo "⚠️  [prv] CAUTION: Security-sensitive files modified in this branch!"
    echo "    Check /tmp/prv-bundle.md before pushing."
  fi
fi

exit 0
```

Make executable:
```bash
chmod +x .git/hooks/pre-push
```
