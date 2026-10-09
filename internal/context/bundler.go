package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vubon/prv/internal/git"
	"github.com/vubon/prv/internal/matrix"
	"github.com/vubon/prv/internal/models"
)

// Bundler coordinates fetching and packaging review context.
type Bundler struct {
	gitRunner *git.GitRunner
	ghClient  *git.GitHubClient
}

// NewBundler creates a new context Bundler.
func NewBundler(repoPath string) *Bundler {
	return &Bundler{
		gitRunner: git.NewGitRunner(repoPath),
		ghClient:  git.NewGitHubClient(),
	}
}

// PrepareBundle gathers all PR metadata, diffs, and filters noise into a structured PRContext.
func (b *Bundler) PrepareBundle(opts models.BundleOptions) (*models.PRContext, error) {
	var meta *models.PRMetadata
	var rawDiff string
	var err error

	// Case 1: Remote GitHub PR URL or Number (Zero-Clone Policy)
	if opts.PRIdentifier != "" && strings.HasPrefix(opts.PRIdentifier, "http") {
		owner, repo, prNum, parseErr := git.ParsePRURL(opts.PRIdentifier)
		if parseErr == nil {
			meta, rawDiff, err = b.ghClient.FetchPRViaAPI(owner, repo, prNum)
			if err != nil {
				// Fallback to gh CLI
				meta, rawDiff, err = b.ghClient.FetchPRViaCLI(opts.PRIdentifier, opts.RepoPath)
			}
		} else {
			meta, rawDiff, err = b.ghClient.FetchPRViaCLI(opts.PRIdentifier, opts.RepoPath)
		}
	} else if opts.PRIdentifier != "" {
		// PR number provided via gh CLI
		meta, rawDiff, err = b.ghClient.FetchPRViaCLI(opts.PRIdentifier, opts.RepoPath)
	}

	// Case 2: Local Git Repository
	if meta == nil {
		if !b.gitRunner.IsGitRepo() {
			return nil, fmt.Errorf("not a git repository and no remote PR specified: %s", opts.RepoPath)
		}

		headBranch := opts.Branch
		if headBranch == "" {
			headBranch, err = b.gitRunner.GetCurrentBranch()
			if err != nil {
				return nil, fmt.Errorf("failed to detect current branch: %w", err)
			}
		}

		baseBranch := opts.BaseBranch
		if baseBranch == "" {
			baseBranch = b.gitRunner.GetDefaultBaseBranch()
		}

		// Try to enrich with GitHub PR description if gh CLI is available
		meta, rawDiff, _ = b.ghClient.FetchPRViaCLI(headBranch, opts.RepoPath)
		if meta == nil {
			meta = &models.PRMetadata{
				Title:       fmt.Sprintf("Branch Review: %s", headBranch),
				Description: "(No PR description found on GitHub — reviewing local branch diff)",
				BaseBranch:  baseBranch,
				HeadBranch:  headBranch,
				CreatedAt:   time.Now(),
			}
		}

		if rawDiff == "" {
			// Fetch diff locally from git
			fileChanges, diffErr := b.gitRunner.GetDiff(baseBranch, headBranch, opts.IncludeLocks)
			if diffErr != nil {
				return nil, fmt.Errorf("failed to get local diff: %w", diffErr)
			}
			return b.buildContext(meta, fileChanges), nil
		}
	}

	if err != nil && rawDiff == "" {
		return nil, fmt.Errorf("failed to fetch PR diff: %w", err)
	}

	// Parse diff string
	fileChanges, err := git.ParseUnifiedDiff(rawDiff, opts.IncludeLocks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	return b.buildContext(meta, fileChanges), nil
}

// buildContext computes stats and builds PRContext.
func (b *Bundler) buildContext(meta *models.PRMetadata, files []models.FileChange) *models.PRContext {
	ctx := &models.PRContext{
		Metadata:  *meta,
		BundledAt: time.Now(),
	}

	stats := models.SummaryStats{
		TotalFiles: len(files),
	}

	for _, f := range files {
		if f.IsFiltered {
			stats.FilteredFiles++
			ctx.IgnoredFiles = append(ctx.IgnoredFiles, fmt.Sprintf("%s (%s)", f.Path, f.FilterReason))
		} else {
			stats.ReviewedFiles++
			stats.TotalAdditions += f.Additions
			stats.TotalDeletions += f.Deletions
			ctx.Files = append(ctx.Files, f)
		}
	}

	ctx.SummaryStats = stats
	return ctx
}

// ToJSON converts PRContext to indented JSON string.
func (b *Bundler) ToJSON(ctx *models.PRContext) (string, error) {
	bytes, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToMarkdown converts PRContext into an LLM-ready markdown prompt bundle.
func (b *Bundler) ToMarkdown(ctx *models.PRContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# PR Review Context: %s\n\n", ctx.Metadata.Title))
	sb.WriteString(fmt.Sprintf("- **Head Branch:** `%s`\n", ctx.Metadata.HeadBranch))
	sb.WriteString(fmt.Sprintf("- **Base Branch:** `%s`\n", ctx.Metadata.BaseBranch))
	if ctx.Metadata.Author != "" {
		sb.WriteString(fmt.Sprintf("- **Author:** @%s\n", ctx.Metadata.Author))
	}
	sb.WriteString(fmt.Sprintf("- **Diff Summary:** %d files reviewed (+%d / -%d)\n",
		ctx.SummaryStats.ReviewedFiles, ctx.SummaryStats.TotalAdditions, ctx.SummaryStats.TotalDeletions))

	if len(ctx.IgnoredFiles) > 0 {
		sb.WriteString(fmt.Sprintf("- **Noise Filtered:** %d files ignored\n", len(ctx.IgnoredFiles)))
	}
	sb.WriteString("\n")

	sb.WriteString("## PR Description / Intent\n")
	sb.WriteString(ctx.Metadata.Description + "\n\n")

	sb.WriteString("## Review Evaluation Matrix\n")
	sb.WriteString(matrix.GenerateMatrixMarkdown() + "\n")

	sb.WriteString("## Code Diffs to Review\n\n")
	for _, file := range ctx.Files {
		sb.WriteString(fmt.Sprintf("### File: `%s` `(+%d / -%d)`\n\n", file.Path, file.Additions, file.Deletions))
		sb.WriteString("```diff\n")
		sb.WriteString(file.DiffHunk)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}
