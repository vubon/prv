package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

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

// GetDefaultBaseBranch attempts to detect origin/main or origin/master.
func (g *GitRunner) GetDefaultBaseBranch() string {
	// Try origin HEAD symbolic ref
	ref, err := g.runCmd("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Try checking if 'main' or 'master' exists locally or remotely
	if _, err := g.runCmd("rev-parse", "--verify", "origin/main"); err == nil {
		return "main"
	}
	if _, err := g.runCmd("rev-parse", "--verify", "main"); err == nil {
		return "main"
	}
	if _, err := g.runCmd("rev-parse", "--verify", "origin/master"); err == nil {
		return "master"
	}
	if _, err := g.runCmd("rev-parse", "--verify", "master"); err == nil {
		return "master"
	}

	return "main"
}

// FetchOrigin runs git fetch to make sure remote branch refs are up-to-date.
func (g *GitRunner) FetchOrigin() error {
	_, err := g.runCmd("fetch", "origin")
	return err
}

// GetDiff runs git diff between base and head and returns parsed file changes.
func (g *GitRunner) GetDiff(baseBranch, headBranch string, includeLocks bool) ([]models.FileChange, error) {
	// Resolve base branch (e.g. origin/main if available)
	baseRef := baseBranch
	if _, err := g.runCmd("rev-parse", "--verify", "origin/"+baseBranch); err == nil {
		baseRef = "origin/" + baseBranch
	}

	headRef := headBranch
	if _, err := g.runCmd("rev-parse", "--verify", "origin/"+headBranch); err == nil {
		headRef = "origin/" + headBranch
	}

	// Run git diff baseRef...headRef
	diffOutput, err := g.runCmd("diff", fmt.Sprintf("%s...%s", baseRef, headRef))
	if err != nil {
		// Fallback to direct two-dot diff or local refs
		diffOutput, err = g.runCmd("diff", fmt.Sprintf("%s..%s", baseBranch, headBranch))
		if err != nil {
			return nil, fmt.Errorf("could not compute diff between %s and %s: %w", baseBranch, headBranch, err)
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
