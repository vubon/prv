package models

import "time"

// PRMetadata holds metadata about the pull request.
type PRMetadata struct {
	Number      int       `json:"number,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Author      string    `json:"author,omitempty"`
	BaseBranch  string    `json:"base_branch"`
	HeadBranch  string    `json:"head_branch"`
	BaseCommit  string    `json:"base_commit,omitempty"`
	HeadCommit  string    `json:"head_commit,omitempty"`
	Repo        string    `json:"repo,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// FileChange represents a single changed file in a PR.
type FileChange struct {
	Path         string `json:"path"`
	OldPath      string `json:"old_path,omitempty"`
	Status       string `json:"status"` // "added", "modified", "deleted", "renamed"
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	DiffHunk     string `json:"diff_hunk"`
	IsFiltered   bool   `json:"is_filtered,omitempty"`
	FilterReason string `json:"filter_reason,omitempty"`
	IsSensitive  bool   `json:"is_sensitive,omitempty"`
	IsTruncated  bool   `json:"is_truncated,omitempty"`
}

// UserContextDoc holds an ingested external reviewer context document (e.g. SPDD canvas, RFC, or spec).
type UserContextDoc struct {
	SourcePath string `json:"source_path"`
	Content    string `json:"content"`
}

// PriorFinding represents a finding from a previous review iteration for re-checking.
type PriorFinding struct {
	ID            string `json:"id"`
	Severity      string `json:"severity"`
	File          string `json:"file"`
	Line          int    `json:"line,omitempty"`
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	Evidence      string `json:"evidence,omitempty"`
	Suggestion    string `json:"suggestion,omitempty"`
	Touched       bool   `json:"touched"`
	PathAmbiguous bool   `json:"path_ambiguous,omitempty"` // Literal backslash or Windows separator could not be disambiguated.
	PathRedacted  bool   `json:"path_redacted,omitempty"`  // File is a display basename without a repository-relative location.
}

// PriorReviewPolicy tells reviewers how to evaluate the prior findings in a bundle.
type PriorReviewPolicy struct {
	Instructions   string   `json:"instructions"`
	Statuses       []string `json:"statuses"`
	StatusGuidance string   `json:"status_guidance"`
	TouchedMeaning string   `json:"touched_meaning"`
}

// ForbiddenEvidenceRequirement is the shared evidence instruction for forbidden rule violations.
const ForbiddenEvidenceRequirement = "Quote the added or deleted lines that introduced the violation, and explain how the resulting code violates the rule."

// RuleEnforcement describes the verdict severity and evidence requirement for a rule tier.
type RuleEnforcement struct {
	Severity            string `json:"severity"`
	EvidenceRequirement string `json:"evidence_requirement,omitempty"`
}

// InvariantRulePolicy describes the severity and evidence contracts for structured team rules.
type InvariantRulePolicy struct {
	Forbidden *RuleEnforcement `json:"forbidden,omitempty"`
	Mandated  *RuleEnforcement `json:"mandated,omitempty"`
}

// PRContext is the complete bundle payload produced by prv bundle.
type PRContext struct {
	Metadata              PRMetadata           `json:"metadata"`
	SummaryStats          SummaryStats         `json:"summary_stats"`
	Files                 []FileChange         `json:"files"`
	FileWalkthroughs      []FileWalkthrough    `json:"file_walkthroughs,omitempty"`
	IgnoredFiles          []string             `json:"ignored_files,omitempty"`
	SensitiveFiles        []string             `json:"sensitive_files,omitempty"`
	CustomRules           []string             `json:"custom_rules,omitempty"`
	ForbiddenRules        []string             `json:"forbidden_rules,omitempty"`
	MandatedRules         []string             `json:"mandated_rules,omitempty"`
	PriorFindings         []PriorFinding       `json:"prior_findings,omitempty"`
	PriorReviewPolicy     *PriorReviewPolicy   `json:"prior_review_policy,omitempty"`
	PriorFindingsOmitted  int                  `json:"prior_findings_omitted,omitempty"`
	PriorReviewPRMismatch bool                 `json:"prior_review_pr_mismatch,omitempty"`
	RulePolicy            *InvariantRulePolicy `json:"rule_policy,omitempty"`
	Persona               string               `json:"persona,omitempty"`
	Instructions          string               `json:"instructions,omitempty"`
	ArchitectureDiagram   string               `json:"architecture_diagram,omitempty"`
	ModifiedConfigFile    string               `json:"modified_config_file,omitempty"`
	BundledAt             time.Time            `json:"bundled_at"`
	UserContext           []UserContextDoc     `json:"user_context,omitempty"`
}

// SummaryStats contains aggregated diff metrics.
type SummaryStats struct {
	TotalFiles     int `json:"total_files"`
	ReviewedFiles  int `json:"reviewed_files"`
	FilteredFiles  int `json:"filtered_files"`
	TruncatedFiles int `json:"truncated_files,omitempty"`
	TotalAdditions int `json:"total_additions"`
	TotalDeletions int `json:"total_deletions"`
}

// BundleOptions holds flags and configuration for the bundle command.
type BundleOptions struct {
	Branch          string
	BaseBranch      string
	PRIdentifier    string // e.g. "42" or "https://github.com/org/repo/pull/42"
	RepoPath        string
	ConfigPath      string // Optional custom path to .prv.yaml
	Staged          bool   // Bundle git staged changes (--cached) for local self-review
	Dirty           bool   // Bundle uncommitted working tree and staged changes (git diff HEAD)
	FormatJSON      bool
	IncludeLocks    bool
	MaxDiffLines    int      // Optional diff line budget cap (--max-lines; 0 = unlimited)
	ContextFiles    []string // Explicit reviewer context file paths (--context-file)
	PriorPath       string   // Optional path to prior review JSON (--prior)
	PersonaOverride string   // Optional CLI persona override (--persona)
}
