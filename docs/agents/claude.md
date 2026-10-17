# Anthropic Claude (Claude Code & Claude Desktop) Setup Guide

This guide explains how to configure **Claude Code CLI** and **Claude Desktop** to perform structured Pull Request reviews using `prv`.

---

## 1. Claude Code CLI Setup (`CLAUDE.md`)

Claude Code automatically reads `CLAUDE.md` in the project root for project-specific instructions and tool guidelines.

Add the following section to your repository's `CLAUDE.md`:

```markdown
# Pull Request & Code Review Workflow (prv)

## Tool Commands
- Bundle current branch against base: `prv bundle --base <base_branch>` (e.g. `staging`, `dev`)
- Bundle staged changes (pre-commit): `prv bundle --staged`
- Bundle branch against custom base: `prv bundle --branch <name> --base <base_branch>`
- Bundle local branch review: `prv bundle --branch <branch_name>`
- Bundle remote GitHub PR: `prv bundle --pr <pr_number_or_url>`
- Generate architecture diagram: `prv diagram --markdown`
- Format review into markdown report: `prv report --file <review_json> --dir reviews/`
- Publish review directly to GitHub PR: `prv report --file <review_json> --pr <id> --post`
- Review starter config: `prv init`

## Review Guidelines
When requested to review code, staged changes, a branch, or a GitHub PR:
1. Always run `prv bundle` first to extract unified diffs, noise-filtered context, and team standards from `.prv.yaml`.
2. Inspect any paths marked `🚨 [SENSITIVE]` for authentication, authorization, or schema migration risks.
3. Evaluate the diffs as a Staff Engineer against the 11 review dimensions:
   - Intent & Spec Alignment
   - Logic & Bug Detection
   - Security & Data Safety
   - Performance & Scalability
   - Testing & Verification
   - Clean Code & Style
   - Architecture & System Coupling
   - Observability & Operability
   - Blast Radius & Failure Modes
   - Data Contract & Migration Safety
   - Environment & Configuration Hygiene
4. Mandatory Sections in Review Report:
   - **🎯 Weakest Link in the Change**
   - **⚖️ Architectural Trade-offs**
   - **🗺️ Architecture & Component Impact Diagram** (Mermaid graph via `prv diagram --markdown` or bundle context)
5. Line-Level Feedback Format:
   Use Conventional Comments (`🚨 blocking`, `⚠️ issue`, `🔒 security`, `⚡ performance`, `💡 suggestion`, `🔍 nit`) with exact line anchors (`file.go#L42`) and copy-ready ` ```suggestion ` blocks.
6. Save the final report to `reviews/<YYYY-MM-DD>-<branch>.md`.
7. **Publishing Safeguard (Human-in-the-Loop):**
   Present findings in terminal or chat first. Do NOT run `prv report --post` or `gh pr comment` until the user explicitly asks to post the comment.
```

---

## 2. Using Claude Code CLI

Once `CLAUDE.md` is in place, you can interact directly via the terminal:

```bash
# Start Claude Code in your repo
claude

# Ask Claude to review:
> review my staged changes before I commit
> review branch feat/payment-retry with prv
> check PR 42 for security bugs and generate markdown report
```

Claude Code will automatically run `prv bundle`, analyze the unified context, and output the staff-level review report.

---

## 3. Claude Desktop Workflow

Claude Desktop interacts with `prv` via file context or local shell tools:

### Option A: Via Generated Bundle File
Generate the review bundle directly to a file:

```bash
# For uncommitted staged changes:
prv bundle --staged -o staged-review.md

# For local branch:
prv bundle --branch feat/my-feature -o review-context.md

# For remote PR:
prv bundle --pr 42 -o pr-42-review.md
```

Drag and drop `staged-review.md` or `review-context.md` into Claude Desktop and prompt:
> "Review this PR context against the 11-dimension engineering matrix. Identify the weakest link, architectural trade-offs, and provide Conventional Comments."

### Option B: Via Local Shell Execution Tool
If Claude Desktop is configured with a local command execution tool:
> "Run `prv bundle --staged` in the repository and evaluate the output against the 11 engineering dimensions."

