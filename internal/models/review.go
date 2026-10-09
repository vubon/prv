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
	Dimension   string `json:"dimension"` // e.g. "Logic", "Security", "Performance"
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
	PRTitle     string       `json:"pr_title"`
	Branch      string       `json:"branch"`
	BaseBranch  string       `json:"base_branch"`
	Verdict     string       `json:"verdict"` // APPROVE, REQUEST_CHANGES, COMMENT
	Summary     string       `json:"summary"`
	RiskLevel   string       `json:"risk_level"`
	Issues      []Issue      `json:"issues"`
	Suggestions []Suggestion `json:"suggestions"`
	ReviewedAt  time.Time    `json:"reviewed_at"`
}

// FormattedComment represents a ready-to-copy GitHub comment block.
type FormattedComment struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	Label        string `json:"label"` // e.g. "⚠️ [bug]", "🔒 [security]", "⚡ [performance]"
	MarkdownBody string `json:"markdown_body"`
}
