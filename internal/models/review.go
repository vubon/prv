package models

import "time"

// Verdict constants
const (
	VerdictApprove        = "APPROVE"
	VerdictRequestChanges = "REQUEST_CHANGES"
	VerdictComment        = "COMMENT"
)

// Risk levels
const (
	RiskLow      = "LOW"
	RiskMedium   = "MEDIUM"
	RiskHigh     = "HIGH"
	RiskCritical = "CRITICAL"
)

// Severity levels
const (
	SeverityBlocking = "BLOCKING"
	SeverityMajor    = "MAJOR"
	SeverityMinor    = "MINOR"
	SeverityNit      = "NIT"
)

// Issue represents a bug, risk, or security finding.
type Issue struct {
	Dimension   string `json:"dimension"`         // e.g. "Logic", "Security", "Performance"
	Subject     string `json:"subject,omitempty"` // Conventional comments subject, e.g. "nil-check", "sqli", "off-by-one"
	File        string `json:"file"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end,omitempty"`
	Severity    string `json:"severity"` // BLOCKING, MAJOR, MINOR, NIT
	Title       string `json:"title"`
	Description string `json:"description"`
	CodeSnippet string `json:"code_snippet,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Suggestion represents an improvement or clean code tip.
type Suggestion struct {
	Dimension   string `json:"dimension"`
	Subject     string `json:"subject,omitempty"` // Conventional comments subject, e.g. "naming", "idiom", "refactor"
	File        string `json:"file"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CodeSnippet string `json:"code_snippet,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ReviewResult represents the structured output of an AI PR review.
type ReviewResult struct {
	PRNumber            int               `json:"pr_number,omitempty"`
	PRURL               string            `json:"pr_url,omitempty"`
	PRTitle             string            `json:"pr_title"`
	Branch              string            `json:"branch"`
	BaseBranch          string            `json:"base_branch"`
	Verdict             string            `json:"verdict"` // APPROVE, REQUEST_CHANGES, COMMENT
	Summary             string            `json:"summary"`
	RiskLevel           string            `json:"risk_level"`
	WeakestLink         string            `json:"weakest_link,omitempty"`
	TradeOffs           string            `json:"trade_offs,omitempty"`
	ArchitectureDiagram string            `json:"architecture_diagram,omitempty"`
	FileWalkthroughs    []FileWalkthrough `json:"file_walkthroughs,omitempty"`
	Issues              []Issue           `json:"issues"`
	Suggestions         []Suggestion      `json:"suggestions"`
	ReviewedAt          time.Time         `json:"reviewed_at"`
}

// FileWalkthrough represents an interactive collapsible summary of changes in a specific file.
type FileWalkthrough struct {
	File        string   `json:"file"`
	Additions   int      `json:"additions"`
	Deletions   int      `json:"deletions"`
	Topic       string   `json:"topic,omitempty"`   // Short thematic label, e.g. "Observability & PATH validation"
	Changes     []string `json:"changes,omitempty"` // Bullet point change descriptions
	IsSensitive bool     `json:"is_sensitive,omitempty"`
}

// FormattedComment represents a ready-to-copy GitHub comment block.
type FormattedComment struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	LineEnd      int    `json:"line_end,omitempty"`
	Label        string `json:"label"` // e.g. "⚠️ [bug]", "🔒 [security]", "⚡ [performance]"
	MarkdownBody string `json:"markdown_body"`
}

// InlineReviewComment represents an inline comment on a specific line of a pull request diff.
type InlineReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side,omitempty"` // "RIGHT" (default) or "LEFT"
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Body      string `json:"body"`
}

// PullRequestReviewPayload represents the payload to create a GitHub Pull Request Review.
type PullRequestReviewPayload struct {
	CommitID string                `json:"commit_id,omitempty"`
	Body     string                `json:"body"`
	Event    string                `json:"event"` // "APPROVE", "REQUEST_CHANGES", "COMMENT"
	Comments []InlineReviewComment `json:"comments,omitempty"`
}
