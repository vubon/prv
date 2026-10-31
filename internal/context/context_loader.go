package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vubon/prv/internal/config"
	"github.com/vubon/prv/internal/git"
	"github.com/vubon/prv/internal/models"
)

var (
	headingRegex   = regexp.MustCompile(`^(\s*)(#{1,6})(\s+.*)$`)
	codeFenceRegex = regexp.MustCompile(`^(\s*)(` + "`" + `{3,}|~{3,})`)
)

// NormalizeMarkdownHeadings shifts markdown headings down by 2 levels (e.g. # -> ###)
// so user-provided specifications do not conflict with root bundle headings,
// while strictly preserving comments and code blocks within fences (including nested code fences).
func NormalizeMarkdownHeadings(content string) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var sb strings.Builder
	inCodeBlock := false
	fenceChar := byte(0)
	fenceLen := 0

	for i, line := range lines {
		// Preserve exact trailing newline behavior without adding trailing newline to empty split
		if i == len(lines)-1 && line == "" && strings.HasSuffix(content, "\n") {
			break
		}

		trimmed := strings.TrimSpace(line)
		if inCodeBlock {
			if len(trimmed) >= fenceLen && trimmed[0] == fenceChar {
				match := true
				for j := 0; j < fenceLen; j++ {
					if trimmed[j] != fenceChar {
						match = false
						break
					}
				}
				if match {
					inCodeBlock = false
					fenceChar = 0
					fenceLen = 0
				}
			}
			sb.WriteString(line)
			sb.WriteString("\n")
			continue
		}

		if codeFenceRegex.MatchString(line) {
			matches := codeFenceRegex.FindStringSubmatch(line)
			if len(matches) >= 3 {
				inCodeBlock = true
				fenceStr := matches[2]
				fenceChar = fenceStr[0]
				fenceLen = len(fenceStr)
			}
			sb.WriteString(line)
			sb.WriteString("\n")
			continue
		}

		if headingRegex.MatchString(line) {
			matches := headingRegex.FindStringSubmatch(line)
			if len(matches) == 4 {
				indent := matches[1]
				hashes := matches[2]
				rest := matches[3]

				newLevel := len(hashes) + 2
				if newLevel > 6 {
					newLevel = 6
				}
				newHashes := strings.Repeat("#", newLevel)
				sb.WriteString(indent + newHashes + rest)
				sb.WriteString("\n")
				continue
			}
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	result := sb.String()
	if !strings.HasSuffix(content, "\n") && strings.HasSuffix(result, "\n") {
		result = strings.TrimSuffix(result, "\n")
	}

	if inCodeBlock && fenceLen > 0 {
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += strings.Repeat(string(fenceChar), fenceLen)
		if strings.HasSuffix(content, "\n") {
			result += "\n"
		}
	}
	return result
}

// LoadContextFiles resolves and reads reviewer-provided context documents based on
// explicit CLI options (--context-file) or auto-discovery from the configured context directory.
func LoadContextFiles(opts models.BundleOptions, cfg *config.Config) ([]models.UserContextDoc, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	contextDir := cfg.Context.Dir
	if contextDir == "" {
		contextDir = ".prv/context"
	}

	maxSizeKB := cfg.Context.MaxSizeKB
	var docs []models.UserContextDoc

	// 1. Explicit Files Provided (--context-file / -c)
	if len(opts.ContextFiles) > 0 {
		for _, path := range opts.ContextFiles {
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".yaml" || ext == ".yml" {
				return nil, fmt.Errorf("context file %q looks like a YAML config; -c now means --context-file (use --config for .prv.yaml)", path)
			}

			resolvedPath := resolveContextFilePath(path, opts.RepoPath, contextDir)
			info, err := os.Stat(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read context file %q: %w", path, err)
			}

			if info.IsDir() {
				return nil, fmt.Errorf("context file %q is a directory, expected a markdown file", path)
			}

			if maxSizeKB > 0 && info.Size() > int64(maxSizeKB)*1024 {
				sizeKB := (info.Size() + 1023) / 1024
				return nil, fmt.Errorf("context file %q exceeds size limit of %d KB (got %d KB); split file or increase context.max_size_kb in .prv.yaml (or set 0 for unlimited)", path, maxSizeKB, sizeKB)
			}

			data, err := os.ReadFile(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read context file %q: %w", path, err)
			}

			normalized := NormalizeMarkdownHeadings(string(data))
			docs = append(docs, models.UserContextDoc{
				SourcePath: path,
				Content:    normalized,
			})
		}
		return docs, nil
	}

	// 2. Auto-Discovery from context.dir
	resolvedDir := resolveFilePath(contextDir, opts.RepoPath)

	dirInfo, err := os.Stat(resolvedDir)
	if err != nil || !dirInfo.IsDir() {
		// Context directory does not exist; auto-discovery safely produces zero docs
		return nil, nil
	}

	candidates := generateCandidateFileNames(opts)
	for _, candidate := range candidates {
		targetFile := filepath.Join(resolvedDir, candidate)
		info, err := os.Stat(targetFile)
		if err == nil && !info.IsDir() {
			if maxSizeKB > 0 && info.Size() > int64(maxSizeKB)*1024 {
				sizeKB := (info.Size() + 1023) / 1024
				fmt.Fprintf(os.Stderr, "Warning: skipping auto-discovered context file %s (%d KB exceeds context.max_size_kb=%d)\n", targetFile, sizeKB, maxSizeKB)
				continue
			}

			data, err := os.ReadFile(targetFile)
			if err == nil {
				relPath, relErr := filepath.Rel(opts.RepoPath, targetFile)
				if relErr != nil {
					relPath = targetFile
				}
				normalized := NormalizeMarkdownHeadings(string(data))
				docs = append(docs, models.UserContextDoc{
					SourcePath: relPath,
					Content:    normalized,
				})
				// Auto-discovery selects the highest precedence matching candidate
				break
			}
		}
	}

	return docs, nil
}

func resolveFilePath(path, repoPath string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if repoPath != "" {
		candidate := filepath.Join(repoPath, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}

func resolveContextFilePath(path, repoPath, contextDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if repoPath != "" {
		candidate := filepath.Join(repoPath, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Fallback: check inside configured context directory (e.g. .prv/context/12.md)
	if contextDir != "" {
		if repoPath != "" {
			candidate := filepath.Join(repoPath, filepath.FromSlash(contextDir), path)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		candidate := filepath.Join(filepath.FromSlash(contextDir), path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if repoPath != "" {
		return filepath.Join(repoPath, path)
	}
	return path
}

func generateCandidateFileNames(opts models.BundleOptions) []string {
	var candidates []string

	// Case A: Remote PR (e.g. 12 or https://github.com/org/repo/pull/12)
	if opts.PRIdentifier != "" {
		id := strings.TrimSpace(opts.PRIdentifier)
		if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
			if _, _, num, err := git.ParsePRURL(id); err == nil {
				id = fmt.Sprintf("%d", num)
			}
		}
		if id != "" {
			candidates = append(candidates,
				fmt.Sprintf("%s.md", id),
				fmt.Sprintf("pr-%s.md", id),
				fmt.Sprintf("pr_%s.md", id),
			)
		}
	}

	// Case B: Staged / Pre-Commit
	if opts.Staged {
		candidates = append(candidates,
			"staged.md",
			"pre-commit.md",
		)
	}

	// Case C: Feature Branch
	branch := opts.Branch
	if branch != "" {
		sanitized := strings.ReplaceAll(branch, "/", "-")
		base := filepath.Base(branch)
		candidates = append(candidates,
			fmt.Sprintf("%s.md", sanitized),
		)
		if base != sanitized {
			candidates = append(candidates, fmt.Sprintf("%s.md", base))
		}
	}

	return candidates
}
