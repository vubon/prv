package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	return g.runCmdAt(g.RepoPath, args...)
}

// runCmdAt executes a git command in an explicit directory.
func (g *GitRunner) runCmdAt(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// GetRepoRoot returns the absolute path of the top-level repository root.
func (g *GitRunner) GetRepoRoot() (string, error) {
	out, err := g.runCmd("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetRepoPrefix returns the working directory prefix relative to the repository root
// (e.g. "services" if run from a subdirectory, or "" if run from the repo root).
func (g *GitRunner) GetRepoPrefix() string {
	out, err := g.runCmd("rev-parse", "--show-prefix")
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(out), "/")
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
	baseRef, _, err := g.ResolveBaseRevision(baseBranch)
	if err != nil {
		return nil, err
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

// IsHexCommitID checks if a string is a 40-character or 64-character full git object ID.
func IsHexCommitID(ref string) bool {
	if len(ref) != 40 && len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func isHexCommitID(ref string) bool {
	return IsHexCommitID(ref)
}

// ResolveBaseRevision resolves baseBranch to an existing commit reference (e.g. origin/<base>, <base>, or SHA),
// attempting a remote fetch if needed. It returns the resolved ref name and the exact commit SHA.
func (g *GitRunner) ResolveBaseRevision(baseBranch string) (refName string, commitSHA string, err error) {
	if err := ValidateRefName(baseBranch); err != nil {
		return "", "", err
	}

	// Direct check if baseBranch is already an exact commit object ID (Finding 3)
	if isHexCommitID(baseBranch) {
		if sha, err := g.runCmd("rev-parse", "--verify", baseBranch+"^{commit}"); err == nil {
			return baseBranch, strings.TrimSpace(sha), nil
		}
		// Attempt to fetch exact commit from origin
		if fetchErr := g.FetchCommit(baseBranch); fetchErr == nil {
			if sha, err := g.runCmd("rev-parse", "--verify", baseBranch+"^{commit}"); err == nil {
				return baseBranch, strings.TrimSpace(sha), nil
			}
		}
		return "", "", fmt.Errorf("commit %q not found locally or on remote origin", baseBranch)
	}

	// Direct check if baseBranch is HEAD or an already qualified ref
	if baseBranch == "HEAD" || strings.HasPrefix(baseBranch, "origin/") || strings.HasPrefix(baseBranch, "refs/") {
		if sha, err := g.runCmd("rev-parse", "--verify", baseBranch+"^{commit}"); err == nil {
			return baseBranch, strings.TrimSpace(sha), nil
		}
		return "", "", fmt.Errorf("revision %q not found locally", baseBranch)
	}

	// Prefer origin/<baseBranch> when available to capture latest upstream changes
	if sha, err := g.runCmd("rev-parse", "--verify", "origin/"+baseBranch+"^{commit}"); err == nil {
		return "origin/" + baseBranch, strings.TrimSpace(sha), nil
	}

	// Check local baseBranch
	if sha, err := g.runCmd("rev-parse", "--verify", baseBranch+"^{commit}"); err == nil {
		return baseBranch, strings.TrimSpace(sha), nil
	}

	// Attempt to fetch origin in case baseBranch was created or updated remotely (15s timeout, non-interactive)
	_, _ = g.runCmdWithTimeout(15*time.Second, []string{"GIT_TERMINAL_PROMPT=0"}, "fetch", "origin", baseBranch)
	if sha, err := g.runCmd("rev-parse", "--verify", "origin/"+baseBranch+"^{commit}"); err == nil {
		return "origin/" + baseBranch, strings.TrimSpace(sha), nil
	}
	if sha, err := g.runCmd("rev-parse", "--verify", baseBranch+"^{commit}"); err == nil {
		return baseBranch, strings.TrimSpace(sha), nil
	}

	return "", "", fmt.Errorf("base branch '%s' not found locally or on remote origin (checked '%s' and 'origin/%s')", baseBranch, baseBranch, baseBranch)
}

// ResolveHeadCommit resolves headBranch (preferring local branch or HEAD, then origin/<headBranch>)
// to its exact commit SHA.
func (g *GitRunner) ResolveHeadCommit(headBranch string) (string, error) {
	headRef := headBranch
	if headRef == "" {
		curr, err := g.GetCurrentBranch()
		if err == nil && curr != "" {
			headRef = curr
		} else {
			headRef = "HEAD"
		}
	}
	if headRef != "HEAD" {
		if err := ValidateRefName(headRef); err != nil {
			return "", err
		}
	}
	if sha, err := g.runCmd("rev-parse", "--verify", headRef+"^{commit}"); err == nil {
		return strings.TrimSpace(sha), nil
	}
	if headRef != "HEAD" && !strings.HasPrefix(headRef, "origin/") && !strings.HasPrefix(headRef, "refs/") && !isHexCommitID(headRef) {
		if sha, err := g.runCmd("rev-parse", "--verify", "origin/"+headRef+"^{commit}"); err == nil {
			return strings.TrimSpace(sha), nil
		}
	}
	return "", fmt.Errorf("could not resolve head revision %q", headBranch)
}

// GetStagedDiff runs git diff --cached and returns parsed file changes for staged index.
func (g *GitRunner) GetStagedDiff(includeLocks bool) ([]models.FileChange, error) {
	diffOutput, err := g.runCmd("diff", "--cached")
	if err != nil {
		return nil, fmt.Errorf("could not compute staged diff: %w", err)
	}

	return ParseUnifiedDiff(diffOutput, includeLocks)
}

// GetWorkingTreeDiff runs git diff HEAD (falling back to unstaged/staged diff on unborn HEAD)
// and returns parsed file changes for uncommitted working tree and staged changes.
func (g *GitRunner) GetWorkingTreeDiff(includeLocks bool) ([]models.FileChange, error) {
	if _, verifyErr := g.runCmd("rev-parse", "--verify", "HEAD"); verifyErr == nil {
		diffOutput, err := g.runCmd("diff", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("could not compute working tree diff: %w", err)
		}
		return ParseUnifiedDiff(diffOutput, includeLocks)
	}

	// Unborn HEAD fallback (repository with no initial commit yet)
	stagedOut, stagedErr := g.runCmd("diff", "--cached")
	unstagedOut, unstagedErr := g.runCmd("diff")
	if stagedErr != nil && unstagedErr != nil {
		return nil, fmt.Errorf("could not compute working tree diff: %w", unstagedErr)
	}
	combined := strings.TrimSpace(stagedOut + "\n" + unstagedOut)
	return ParseUnifiedDiff(combined, includeLocks)
}

// resolveCommitRef resolves ref (or origin/<ref> for unqualified branch names) to a valid commit ref,
// returning "" if neither exists locally. It prefers exact object IDs, then origin/<ref> for unqualified branch names.
func (g *GitRunner) resolveCommitRef(ref string) string {
	if err := ValidateRefName(ref); err != nil {
		return ""
	}
	if isHexCommitID(ref) {
		if _, err := g.runCmd("rev-parse", "--verify", ref+"^{commit}"); err == nil {
			return ref
		}
		return ""
	}
	if ref == "HEAD" || strings.HasPrefix(ref, "origin/") || strings.HasPrefix(ref, "refs/") {
		if _, err := g.runCmd("rev-parse", "--verify", ref+"^{commit}"); err == nil {
			return ref
		}
		return ""
	}
	if _, err := g.runCmd("rev-parse", "--verify", "origin/"+ref+"^{commit}"); err == nil {
		return "origin/" + ref
	}
	if _, err := g.runCmd("rev-parse", "--verify", ref+"^{commit}"); err == nil {
		return ref
	}
	return ""
}

// RefExists reports whether ref (or origin/<ref>) resolves to an existing commit in the local repository.
func (g *GitRunner) RefExists(ref string) bool {
	return g.resolveCommitRef(ref) != ""
}

// FetchCommit attempts to fetch an exact commit object from origin (15s timeout, non-interactive).
func (g *GitRunner) FetchCommit(sha string) error {
	if !isHexCommitID(sha) {
		return fmt.Errorf("invalid commit sha %q", sha)
	}
	_, err := g.runCmdWithTimeout(15*time.Second, []string{"GIT_TERMINAL_PROMPT=0"}, "fetch", "origin", sha)
	return err
}

// resolveCommittedPath resolves filePath within commit ref, following both file symlinks
// and intermediate directory symlinks up to maxHops. It returns the final blob SHA, the final resolved
// repository-relative path, all traversed repository-relative paths, found bool, and any error encountered.
func (g *GitRunner) resolveCommittedPath(ref, filePath string, maxHops int) (blobSHA string, resolvedPath string, traversedPaths []string, found bool, err error) {
	rawPath := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(filePath)), "./")
	if rawPath == "" {
		return "", "", nil, false, nil
	}

	repoPrefix := g.GetRepoPrefix()
	repoRelPath := rawPath
	if repoPrefix != "" {
		if !strings.HasPrefix(rawPath, repoPrefix+"/") && rawPath != repoPrefix {
			repoRelPath = filepath.Clean(filepath.Join(repoPrefix, rawPath))
			repoRelPath = strings.TrimPrefix(filepath.ToSlash(repoRelPath), "./")
		}
	}

	if strings.HasPrefix(repoRelPath, "..") || strings.Contains(repoRelPath, "/../") {
		return "", "", nil, false, fmt.Errorf("repository file path %q points outside repository", filePath)
	}

	repoRoot, _ := g.GetRepoRoot()
	if repoRoot == "" {
		repoRoot = g.RepoPath
	}

	traversedPaths = append(traversedPaths, repoRelPath)
	currentPath := repoRelPath

	hops := 0
	for hops < maxHops {
		segments := strings.Split(currentPath, "/")
		var prefix string
		foundSymlink := false

		for i, seg := range segments {
			if prefix == "" {
				prefix = seg
			} else {
				prefix = prefix + "/" + seg
			}

			lsOut, lsErr := g.runCmdAt(repoRoot, "ls-tree", ref, "--", prefix)
			if lsErr != nil {
				return "", "", nil, false, fmt.Errorf("failed to read git tree at %s:%s: %w", ref, prefix, lsErr)
			}
			trimmedOut := strings.TrimSpace(lsOut)
			if trimmedOut == "" {
				return "", "", nil, false, nil
			}

			fields := strings.Fields(trimmedOut)
			if len(fields) < 3 {
				return "", "", nil, false, fmt.Errorf("invalid ls-tree output for %s:%s: %q", ref, prefix, trimmedOut)
			}
			mode := fields[0]
			objType := fields[1]
			sha := fields[2]

			if mode == "120000" {
				hops++
				if hops >= maxHops {
					return "", "", nil, false, fmt.Errorf("too many symlink levels resolving %q at ref %q", filePath, ref)
				}
				targetBytes, catErr := g.runCmdAt(repoRoot, "cat-file", "-p", sha)
				if catErr != nil {
					return "", "", nil, false, fmt.Errorf("failed to read symlink target at %s:%s: %w", ref, prefix, catErr)
				}
				targetRaw := strings.TrimSpace(targetBytes)
				if strings.HasPrefix(targetRaw, "/") || strings.HasPrefix(targetRaw, "\\") || filepath.IsAbs(targetRaw) || strings.Contains(targetRaw, ":") {
					return "", "", nil, false, fmt.Errorf("tracked symlink %q at ref %q points outside repository root (%q)", prefix, ref, targetRaw)
				}
				dir := filepath.Dir(prefix)
				if dir == "." {
					dir = ""
				}
				targetPath := filepath.Clean(filepath.Join(dir, targetRaw))
				targetPath = strings.TrimPrefix(filepath.ToSlash(targetPath), "./")
				if targetPath == ".." || strings.HasPrefix(targetPath, "../") || strings.HasPrefix(targetPath, "/") || strings.Contains(targetPath, ":") {
					return "", "", nil, false, fmt.Errorf("tracked symlink %q at ref %q points outside repository root (%q)", prefix, ref, targetRaw)
				}

				traversedPaths = append(traversedPaths, prefix)
				traversedPaths = append(traversedPaths, targetPath)

				remaining := segments[i+1:]
				if len(remaining) > 0 {
					currentPath = filepath.Clean(filepath.Join(targetPath, filepath.Join(remaining...)))
					currentPath = strings.TrimPrefix(filepath.ToSlash(currentPath), "./")
				} else {
					currentPath = targetPath
				}
				foundSymlink = true
				break
			}

			if i == len(segments)-1 {
				if objType == "blob" {
					traversedPaths = append(traversedPaths, prefix)
					dedup := make([]string, 0, len(traversedPaths))
					seen := make(map[string]bool)
					for _, p := range traversedPaths {
						cleanP := strings.TrimPrefix(filepath.ToSlash(p), "./")
						if cleanP != "" && !seen[cleanP] {
							seen[cleanP] = true
							dedup = append(dedup, cleanP)
						}
					}
					return sha, prefix, dedup, true, nil
				}
				return "", "", nil, false, nil
			}

			if objType != "tree" {
				return "", "", nil, false, nil
			}
		}

		if !foundSymlink {
			break
		}
	}

	return "", "", nil, false, nil
}

// ReadFileAtRef reads the exact bytes of filePath at the given git commit/branch ref,
// resolving tracked git symlinks (mode 120000) and directory symlinks within the same revision up to 5 hops.
// It returns found=false, err=nil when the ref or blob path does not exist at that revision.
func (g *GitRunner) ReadFileAtRef(ref, filePath string) ([]byte, bool, error) {
	bytes, _, _, found, err := g.ReadFileAtRefWithResolvedPath(ref, filePath)
	return bytes, found, err
}

// ReadFileAtRefWithResolvedPath reads the exact bytes of filePath at the given git commit/branch ref,
// returning the final resolved in-tree path and all traversed paths alongside the blob bytes.
func (g *GitRunner) ReadFileAtRefWithResolvedPath(ref, filePath string) ([]byte, string, []string, bool, error) {
	if err := ValidateRefName(ref); err != nil {
		return nil, "", nil, false, err
	}
	cleanPath := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(filePath)), "./")
	if cleanPath == "" || strings.ContainsAny(cleanPath, ":\r\n") {
		return nil, "", nil, false, fmt.Errorf("invalid repository file path %q", filePath)
	}

	resolved := g.resolveCommitRef(ref)
	if resolved == "" {
		return nil, "", nil, false, nil
	}

	const maxSymlinkHops = 5
	blobSHA, resolvedPath, traversedPaths, found, err := g.resolveCommittedPath(resolved, cleanPath, maxSymlinkHops)
	if err != nil {
		return nil, "", nil, false, err
	}
	if !found {
		return nil, "", nil, false, nil
	}

	repoRoot, _ := g.GetRepoRoot()
	if repoRoot == "" {
		repoRoot = g.RepoPath
	}
	cmd := exec.Command("git", "cat-file", "-p", blobSHA)
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, "", nil, false, fmt.Errorf("git cat-file -p %s failed: %w (stderr: %s)", blobSHA, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), resolvedPath, traversedPaths, true, nil
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
		} else if strings.HasPrefix(line, "similarity index") {
			currentFile.Status = "renamed"
		} else if strings.HasPrefix(line, "rename from ") {
			currentFile.Status = "renamed"
			oldP := strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
			currentFile.OldPath = strings.Trim(oldP, "\"")
		} else if strings.HasPrefix(line, "rename to ") {
			currentFile.Status = "renamed"
			newP := strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
			currentFile.Path = strings.Trim(newP, "\"")
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
