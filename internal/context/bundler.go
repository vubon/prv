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
	// Case 1: Remote GitHub PR URL or Number (Zero-Clone Policy)
	if opts.PRIdentifier != "" {
		var meta *models.PRMetadata
		var rawDiff string
		var err error

		if strings.HasPrefix(opts.PRIdentifier, "http") {
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
		} else {
			// PR number provided via gh CLI
			meta, rawDiff, err = b.ghClient.FetchPRViaCLI(opts.PRIdentifier, opts.RepoPath)
		}

		if err != nil || meta == nil {
			if err == nil {
				err = fmt.Errorf("could not retrieve PR metadata")
			}
			return nil, fmt.Errorf("failed to fetch remote PR %s: %w", opts.PRIdentifier, err)
		}

		fileChanges, parseErr := git.ParseUnifiedDiff(rawDiff, opts.IncludeLocks)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse PR diff: %w", parseErr)
		}

		return b.buildContext(meta, fileChanges), nil
	}

	// Case 2: Local Git Repository
	if !b.gitRunner.IsGitRepo() {
		return nil, fmt.Errorf("not a git repository and no remote PR specified: %s", opts.RepoPath)
	}

	headBranch := opts.Branch
	var err error
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

	var meta *models.PRMetadata
	var fileChanges []models.FileChange

	// If user explicitly specified a custom base branch, compute diff locally against that base
	if opts.BaseBranch != "" {
		ghMeta, _, _ := b.ghClient.FetchPRViaCLI(headBranch, opts.RepoPath)
		if ghMeta != nil {
			meta = ghMeta
			meta.BaseBranch = baseBranch
		}

		localChanges, diffErr := b.gitRunner.GetDiff(baseBranch, headBranch, opts.IncludeLocks)
		if diffErr != nil {
			return nil, fmt.Errorf("failed to get local diff against base %s: %w", baseBranch, diffErr)
		}
		fileChanges = localChanges
	} else {
		// Try to enrich with GitHub PR description & diff if gh CLI is available
		ghMeta, ghDiff, _ := b.ghClient.FetchPRViaCLI(headBranch, opts.RepoPath)
		if ghMeta != nil && ghDiff != "" {
			meta = ghMeta
			parsedChanges, parseErr := git.ParseUnifiedDiff(ghDiff, opts.IncludeLocks)
			if parseErr == nil {
				fileChanges = parsedChanges
			}
		}

		// Fallback to local git diff if GitHub PR was not found or had empty diff
		if len(fileChanges) == 0 && meta == nil {
			localChanges, diffErr := b.gitRunner.GetDiff(baseBranch, headBranch, opts.IncludeLocks)
			if diffErr != nil {
				return nil, fmt.Errorf("failed to get local diff: %w", diffErr)
			}
			fileChanges = localChanges
		}
	}

	if meta == nil {
		desc := "(No PR description found on GitHub — reviewing local branch diff)"
		if headBranch == baseBranch {
			desc += fmt.Sprintf("\n\nNote: Head branch '%s' is identical to base branch '%s' (0 changes).", headBranch, baseBranch)
		}
		meta = &models.PRMetadata{
			Title:       fmt.Sprintf("Branch Review: %s", headBranch),
			Description: desc,
			BaseBranch:  baseBranch,
			HeadBranch:  headBranch,
			CreatedAt:   time.Now(),
		}
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
