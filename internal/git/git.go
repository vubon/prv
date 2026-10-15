package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vubon/prv/internal/models"
)

// GitRunner handles interaction with the local git command line.
type GitRunner struct {
	RepoPath string
}

// NewGitRunner creates a runner for the specified repo directory.
func NewGitRunner(repoPath string) *GitRunner {
	if repoPath == "" {
		repoPath = "."
	}
	return &GitRunner{RepoPath: repoPath}
}

// runCmd executes a git command in the repository path.
func (g *GitRunner) runCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// runCmdWithTimeout executes a git command in the repository path with an explicit timeout
// and optional environment variables.
func (g *GitRunner) runCmdWithTimeout(timeout time.Duration, extraEnv []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.RepoPath
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ValidateRefName checks that a branch or ref name is safe to pass to git arguments
// to prevent option injection (e.g. names starting with '-') or illegal characters.
func ValidateRefName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("branch or revision name cannot be empty")
	}
	if strings.HasPrefix(trimmed, "-") {
		return fmt.Errorf("invalid branch name %q: cannot start with a dash", name)
	}
	if strings.ContainsAny(trimmed, " \t\n~^:?*[\\") || strings.Contains(trimmed, "..") {
		return fmt.Errorf("invalid branch name %q: contains illegal git ref characters", name)
	}
	return nil
}

// IsGitRepo checks if the path is a valid git repository.
func (g *GitRunner) IsGitRepo() bool {
	_, err := g.runCmd("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// GetCurrentBranch returns the active git branch name.
func (g *GitRunner) GetCurrentBranch() (string, error) {
	return g.runCmd("rev-parse", "--abbrev-ref", "HEAD")
}

// GetRemoteURL returns the remote.origin.url of the repository.
func (g *GitRunner) GetRemoteURL() (string, error) {
	return g.runCmd("config", "--get", "remote.origin.url")
}

// GetDefaultBaseBranch attempts to detect default base branch (symbolic-ref, main, master, develop, dev, staging).
func (g *GitRunner) GetDefaultBaseBranch() string {
	// Try origin HEAD symbolic ref
	ref, err := g.runCmd("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Try checking common default branch names in order of prevalence
	candidates := []string{"main", "master", "develop", "dev", "staging"}
	for _, candidate := range candidates {
		if _, err := g.runCmd("rev-parse", "--verify", "origin/"+candidate); err == nil {
			return candidate
		}
		if _, err := g.runCmd("rev-parse", "--verify", candidate); err == nil {
			return candidate
		}
	}

	return "main"
}

// FetchOrigin runs git fetch with a 15-second timeout and disables interactive terminal prompts.
func (g *GitRunner) FetchOrigin() error {
	_, err := g.runCmdWithTimeout(15*time.Second, []string{"GIT_TERMINAL_PROMPT=0"}, "fetch", "origin")
	return err
}

// GetDiff runs git diff between base and head and returns parsed file changes.
func (g *GitRunner) GetDiff(baseBranch, headBranch string, includeLocks bool) ([]models.FileChange, error) {
	if headBranch != "" && headBranch != "HEAD" {
		if err := ValidateRefName(headBranch); err != nil {
			return nil, err
		}
	}
	if baseBranch != "" {
		if err := ValidateRefName(baseBranch); err != nil {
			return nil, err
		}
	}

	// Resolve head branch ref:
	// Prioritize local branch if it exists (or HEAD) so unpushed local commits are included
	headRef := headBranch
	if headRef == "" {
		curr, err := g.GetCurrentBranch()
		if err == nil && curr != "" {
			headRef = curr
		} else {
			headRef = "HEAD"
		}
	}

	// If headRef does not exist locally as a branch or ref, check if origin/<headRef> exists
	if _, err := g.runCmd("rev-parse", "--verify", headRef); err != nil && headRef != "HEAD" {
		if _, err := g.runCmd("rev-parse", "--verify", "origin/"+headRef); err == nil {
			headRef = "origin/" + headRef
		}
	}

	// Resolve base branch ref:
	if baseBranch == "" {
		baseBranch = g.GetDefaultBaseBranch()
	}
	if err := ValidateRefName(baseBranch); err != nil {
		return nil, err
	}
	baseRef := baseBranch

	// Check if base branch exists remotely or locally
	if _, err := g.runCmd("rev-parse", "--verify", "origin/"+baseRef); err == nil {
		baseRef = "origin/" + baseRef
	} else if _, err := g.runCmd("rev-parse", "--verify", baseRef); err == nil {
		// Local branch / ref exists
	} else {
		// Attempt to fetch origin in case baseBranch was created or updated remotely (15s timeout, non-interactive)
		_, _ = g.runCmdWithTimeout(15*time.Second, []string{"GIT_TERMINAL_PROMPT=0"}, "fetch", "origin", baseBranch)
		if _, err := g.runCmd("rev-parse", "--verify", "origin/"+baseBranch); err == nil {
			baseRef = "origin/" + baseBranch
		} else if _, err := g.runCmd("rev-parse", "--verify", baseBranch); err == nil {
			baseRef = baseBranch
		} else {
			return nil, fmt.Errorf("base branch '%s' not found locally or on remote origin (checked '%s' and 'origin/%s')", baseBranch, baseBranch, baseBranch)
		}
	}

	// Run git diff baseRef...headRef (three dots finds merge base)
	diffOutput, err := g.runCmd("diff", fmt.Sprintf("%s...%s", baseRef, headRef))
	if err != nil {
		// Fallback to direct two-dot diff or local refs (handles disconnected root commits or shallow clones)
		diffOutput, err = g.runCmd("diff", fmt.Sprintf("%s..%s", baseRef, headRef))
		if err != nil {
			diffOutput, err = g.runCmd("diff", fmt.Sprintf("%s..%s", baseBranch, headBranch))
			if err != nil {
				return nil, fmt.Errorf("could not compute diff between base '%s' and head '%s': %w", baseBranch, headBranch, err)
			}
		}
	}

	return ParseUnifiedDiff(diffOutput, includeLocks)
}

// GetStagedDiff runs git diff --cached and returns parsed file changes for staged index.
func (g *GitRunner) GetStagedDiff(includeLocks bool) ([]models.FileChange, error) {
	diffOutput, err := g.runCmd("diff", "--cached")
	if err != nil {
		return nil, fmt.Errorf("could not compute staged diff: %w", err)
	}

	return ParseUnifiedDiff(diffOutput, includeLocks)
}

// ParseUnifiedDiff parses a raw unified diff string into structured FileChange objects.
func ParseUnifiedDiff(rawDiff string, includeLocks bool) ([]models.FileChange, error) {
	var files []models.FileChange
	if strings.TrimSpace(rawDiff) == "" {
		return files, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(rawDiff))
	// Allocate 1MB initial buffer, expanding up to 10MB to safely handle large diff lines
	const maxLineSize = 10 * 1024 * 1024
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, maxLineSize)

	var currentFile *models.FileChange
	var hunkBuilder strings.Builder
	inHunk := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			// Save previous file if exists
			if currentFile != nil {
				currentFile.DiffHunk = hunkBuilder.String()
				files = append(files, *currentFile)
				hunkBuilder.Reset()
			}
			inHunk = false

			// Parse file path from: diff --git a/path/to/file b/path/to/file
			header := strings.TrimPrefix(line, "diff --git ")
			var filePath string
			if idx := strings.Index(header, " b/"); idx != -1 {
				filePath = header[idx+3:]
			} else if parts := strings.Split(header, " "); len(parts) >= 2 {
				filePath = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
			filePath = strings.Trim(filePath, "\"")

			filtered, reason := false, ""
			if !includeLocks {
				filtered, reason = ShouldFilterFile(filePath)
			}

			currentFile = &models.FileChange{
				Path:         filePath,
				Status:       "modified",
				IsFiltered:   filtered,
				FilterReason: reason,
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		if strings.HasPrefix(line, "new file mode") {
			currentFile.Status = "added"
		} else if strings.HasPrefix(line, "deleted file mode") {
			currentFile.Status = "deleted"
		} else if strings.HasPrefix(line, "similarity index") || strings.HasPrefix(line, "rename from") {
			currentFile.Status = "renamed"
		} else if strings.HasPrefix(line, "+++ ") && !inHunk {
			target := strings.TrimPrefix(line, "+++ ")
			target = strings.TrimPrefix(target, "b/")
			target = strings.Trim(target, "\"")
			if target != "/dev/null" && target != "" && currentFile.Path == "" {
				currentFile.Path = target
			}
		}

		if strings.HasPrefix(line, "@@ ") {
			inHunk = true
		}

		// Count additions and deletions strictly inside hunks
		if inHunk {
			if strings.HasPrefix(line, "+") {
				currentFile.Additions++
			} else if strings.HasPrefix(line, "-") {
				currentFile.Deletions++
			}

			// Collect hunk content if file is not filtered
			if !currentFile.IsFiltered {
				hunkBuilder.WriteString(line)
				hunkBuilder.WriteString("\n")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return files, fmt.Errorf("error reading unified diff: %w", err)
	}

	if currentFile != nil {
		currentFile.DiffHunk = hunkBuilder.String()
		files = append(files, *currentFile)
	}

	return files, nil
}

// GetCommitLog returns the list of commits between base and head.
func (g *GitRunner) GetCommitLog(baseBranch, headBranch string) (string, error) {
	return g.runCmd("log", fmt.Sprintf("origin/%s..origin/%s", baseBranch, headBranch), "--oneline", "-n", "20")
}
