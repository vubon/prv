# Headless Terminal & CLI Pipeline Guide

`prv` is built according to Unix philosophy: it produces clean, structured markdown or JSON to stdout, making it directly composable with terminal LLM clients, automation scripts, and Git hooks.

---

## 1. Piping into Terminal LLMs

### Using `llm` (Simon Willison's CLI)

```bash
# Review uncommitted staged changes before committing (Shift-Left)
prv bundle --staged | llm -m claude-3-7-sonnet \
  -s "Review these staged changes before I commit. Flag any bugs, missing tests, or security sensitive paths."

# Review current branch with Claude 3.7 Sonnet
prv bundle --branch feat/auth | llm -m claude-3-7-sonnet \
  -s "You are a Staff Engineer reviewing this PR context. Identify bugs, the weakest link, and architectural trade-offs."

# Review remote GitHub PR with GPT-4o
prv bundle --pr 42 | llm -m gpt-4o \
  -s "Review this PR against the 9-dimension checklist and provide Conventional Comments."
```

### Using `aichat`

```bash
prv bundle --staged | aichat -r staff-reviewer
prv bundle --branch feat/cache | aichat -r staff-reviewer
```

### Using GitHub Copilot CLI (`gh copilot`)

```bash
prv bundle --staged --json | gh copilot explain
prv bundle --branch feat/billing --json | gh copilot explain
```

---

## 2. Terminal Review Helper Script

You can add a simple helper function to your `~/.zshrc` or `~/.bashrc`:

```bash
# Review local branch, PR, or staged changes with prv + LLM
prv-review() {
  local target="${1:-$(git rev-parse --abbrev-ref HEAD)}"
  
  if [[ "$target" == "--staged" ]] || [[ "$target" == "-s" ]]; then
    echo "🔍 Bundling pre-commit staged changes ..."
    prv bundle --staged | llm -m claude-3-7-sonnet
  elif [[ "$target" =~ ^[0-9]+$ ]] || [[ "$target" =~ ^https:// ]]; then
    echo "🔍 Bundling review context for PR: $target ..."
    prv bundle --pr "$target" | llm -m claude-3-7-sonnet
  else
    echo "🔍 Bundling review context for branch: $target ..."
    prv bundle --branch "$target" | llm -m claude-3-7-sonnet
  fi
}
```

Usage:
```bash
prv-review --staged         # Reviews uncommitted staged changes
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

```bash
chmod +x .git/hooks/pre-push
```

---

## 4. Git Pre-Commit Hook Integration

To automatically detect sensitive security paths before every commit using `prv bundle --staged`:

Create `.git/hooks/pre-commit`:

```bash
#!/usr/bin/env bash
set -e

# Check staged files against .prv.yaml sensitive boundaries
if ! prv bundle --staged --output /tmp/prv-staged.md > /dev/null 2>&1; then
  exit 0
fi

if grep -q "Security Sensitive Paths Touched" /tmp/prv-staged.md; then
  echo "⚠️  [prv] CAUTION: Security-sensitive files staged for commit!"
  echo "    Review details in /tmp/prv-staged.md before committing."
fi

exit 0
```

Make executable:
```bash
chmod +x .git/hooks/pre-commit
```
