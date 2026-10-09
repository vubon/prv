# Conventional Comments Specification

`prv` review output format adheres to the **Conventional Comments** standard to provide clear, actionable, and empathetic review feedback.

## Format

```markdown
<label> [decorations] (<subject>): <discussion>
```

Followed optionally by a GitHub suggestion block (` ```suggestion `).

## Labels

| Label | Meaning | Blocking? | Example |
| :--- | :--- | :--- | :--- |
| **`🚨 blocking`** | Critical flaw, security risk, or broken logic that must be resolved before merge. | Yes | `🚨 blocking (nil-check): sess may be nil on cache miss.` |
| **`⚠️ issue`** | Bug or incorrect logic that needs fixing. | Yes | `⚠️ issue (off-by-one): Slice index exceeds length.` |
| **`🔒 security`** | Vulnerability, injection risk, or secret exposure. | Yes | `🔒 security (sqli): String concatenation in query.` |
| **`⚡ performance`**| Sub-optimal performance, N+1 query, or memory leak. | Medium | `⚡ performance (db): Query inside loop should be batched.` |
| **`💡 suggestion`** | Actionable improvement to design, naming, or clarity. | No | `💡 suggestion (naming): Rename to isEnabled for clarity.` |
| **`🔍 nit`** | Minor cosmetic or stylistic preference. | No | `🔍 nit: Trailing whitespace.` |
| **`❓ question`** | Inquiry to understand rationale or context. | No | `❓ question (context): Why do we override this timeout?` |
| **`👏 praise`** | Positive feedback acknowledging good patterns. | No | `👏 praise: Clean refactor of the payment state machine!` |

## Example Comment Output

```markdown
**File:** `internal/session/cache.go#L54`  
**Label:** `⚠️ [bug]`

> **issue (nil-check):** If `cache.Get()` returns a cache miss with `err == nil`, `sess` is `nil` and will cause a runtime panic.
> 
> ```suggestion
> sess, err := c.store.Get(ctx, key)
> if err != nil {
>     return nil, err
> }
> if sess == nil {
>     return nil, ErrSessionNotFound
> }
> ```
```
