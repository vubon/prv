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
}

// PRContext is the complete bundle payload produced by prv bundle.
type PRContext struct {
	Metadata            PRMetadata        `json:"metadata"`
	SummaryStats        SummaryStats      `json:"summary_stats"`
	Files               []FileChange      `json:"files"`
	FileWalkthroughs    []FileWalkthrough `json:"file_walkthroughs,omitempty"`
	IgnoredFiles        []string          `json:"ignored_files,omitempty"`
	SensitiveFiles      []string          `json:"sensitive_files,omitempty"`
	CustomRules         []string          `json:"custom_rules,omitempty"`
	Persona             string            `json:"persona,omitempty"`
	ArchitectureDiagram string            `json:"architecture_diagram,omitempty"`
	BundledAt           time.Time         `json:"bundled_at"`
}

// SummaryStats contains aggregated diff metrics.
type SummaryStats struct {
	TotalFiles     int `json:"total_files"`
	ReviewedFiles  int `json:"reviewed_files"`
	FilteredFiles  int `json:"filtered_files"`
	TotalAdditions int `json:"total_additions"`
	TotalDeletions int `json:"total_deletions"`
}

// BundleOptions holds flags and configuration for the bundle command.
type BundleOptions struct {
	Branch       string
	BaseBranch   string
	PRIdentifier string // e.g. "42" or "https://github.com/org/repo/pull/42"
	RepoPath     string
	ConfigPath   string // Optional custom path to .prv.yaml
	Staged       bool   // Bundle git staged changes (--cached) for local self-review
	FormatJSON   bool
	IncludeLocks bool
	MaxDiffLines int
	CloneCache   bool
}
