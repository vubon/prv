# Google Antigravity Setup Guide

This guide explains how to configure **Google Antigravity** to act as an autonomous Staff Engineer reviewer using the `prv` context engine and KacheDB memory.

---

## 1. Directory Structure

In your repository root, establish the `.agents/` customization folder:

```
.agents/
├── AGENTS.md                  # Workspace behavioral rules and pre-commit gates
└── skills/
    └── prv/
        └── SKILL.md           # On-demand PR review skill instructions
```

---

## 2. Workspace Rules (`.agents/AGENTS.md`)

Create or update `.agents/AGENTS.md` with mandatory instructions for Antigravity:

```markdown
# AGENTS.md — prv Autonomous Review & Memory Rules

## 🔴 MANDATORY PRE-REVIEW CACHE CHECK
Before analyzing raw diffs or re-evaluating existing PRs:
1. Check KacheDB for prior cached reviews:
   `call_mcp_tool(ServerName="kachedb", ToolName="kache_semantic_search", Arguments={"query": "<pr_title_or_branch>", "exact_first": true})`
2. If prior findings or architectural context exist, reuse them to avoid redundant LLM computation.

## 🟢 MANDATORY PR BUNDLE WORKFLOW
When requested to review a branch or Pull Request:
1. Generate the review bundle via prv:
   - For local branch: `./bin/prv bundle --branch <branch_name>`
   - For remote GitHub PR: `./bin/prv bundle --pr <pr_url_or_number>`
   - For custom rules: `./bin/prv bundle --config .prv.yaml`
2. Evaluate the bundle against the 9-Dimension Engineering Evaluation Matrix.
3. Every review report MUST explicitly highlight:
   - **🎯 The Weakest Link in the Change** (how the change will fail under stress)
   - **⚖️ Architectural Trade-offs** (what compromises were made)
4. Format line-level findings using Conventional Comments (`🚨 blocking`, `⚠️ issue`, `🔒 security`, `⚡ performance`, `💡 suggestion`, `🔍 nit`) with exact file paths (`path/file.go#L42`) and copy-ready ` ```suggestion ` blocks.
5. Save the report to `reviews/<YYYY-MM-DD>-<branch>.md`.

## 💾 POST-REVIEW KNOWLEDGE PERSISTENCE
In the same turn, persist findings into KacheDB:
`call_mcp_tool(ServerName="kachedb", ToolName="kache_save_context", Arguments={"topic": "<pr_topic_slug>", "prompt_alias": "<user_query>", "content": "<summary>", "tags": ["pr-review", "prv"]})`
```

---

## 3. Skill Definition (`.agents/skills/prv/SKILL.md`)

Create `.agents/skills/prv/SKILL.md` to register `prv` as an on-demand skill:

```markdown
---
name: prv
description: Autonomous Pull Request reviewer using the prv Go context engine and 9-dimension engineering matrix
---

# PR Review Context Engine (prv)

## Triggering Reviews
When the user asks to review code:
1. Run `prv bundle --branch <name>` or `prv bundle --pr <number>`.
2. Review unified diffs with active team standards from `.prv.yaml`.
3. Format output per the 9 dimensions and Conventional Comments.
4. Save report in `reviews/` and persist summary to KacheDB.
```

---

## 4. Usage in Antigravity Chat

Once configured, invoke Antigravity with natural prompts:

```
"Review branch feat/payment-gateway"
"Review PR #42 with prv"
"Check our sensitive paths in PR https://github.com/org/repo/pull/10"
```
