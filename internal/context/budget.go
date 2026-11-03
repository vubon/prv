package context

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vubon/prv/internal/models"
)

const (
	riskTierSensitive = 1
	riskTierStandard  = 2
	riskTierLow       = 3
)

// ApplyDiffBudget enforces an optional maximum diff line cap across files using
// risk-tiered prioritization:
//   - Tier 1 (Highest priority): Security-sensitive files (IsSensitive == true)
//   - Tier 2 (Standard priority): Core source code files
//   - Tier 3 (Lowest priority): Test files and documentation
//
// Original file ordering in the returned slice is preserved. When maxLines <= 0,
// no truncation is performed.
func ApplyDiffBudget(files []models.FileChange, maxLines int) ([]models.FileChange, int) {
	if maxLines <= 0 || len(files) == 0 {
		return files, 0
	}

	lineCounts := make([]int, len(files))
	totalLines := 0
	for i := range files {
		c := countDiffLines(files[i].DiffHunk)
		lineCounts[i] = c
		totalLines += c
	}

	if totalLines <= maxLines {
		return files, 0
	}

	out := make([]models.FileChange, len(files))
	copy(out, files)

	remaining := maxLines
	truncatedCount := 0

	for tier := riskTierSensitive; tier <= riskTierLow; tier++ {
		for i := range out {
			if classifyFileRiskTier(out[i]) != tier {
				continue
			}

			count := lineCounts[i]
			if count == 0 {
				continue
			}

			if count <= remaining {
				remaining -= count
				continue
			}

			if remaining > 0 {
				lines := strings.Split(strings.TrimSuffix(out[i].DiffHunk, "\n"), "\n")
				kept := strings.Join(lines[:remaining], "\n")
				out[i].DiffHunk = fmt.Sprintf(
					"%s\n... [diff truncated at --max-lines budget (%d/%d lines shown); pass --max-lines 0 for full diff]\n",
					kept, remaining, count,
				)
				out[i].IsTruncated = true
				truncatedCount++
				remaining = 0
			} else {
				out[i].DiffHunk = fmt.Sprintf(
					"... [diff omitted due to --max-lines budget (%d lines); pass --max-lines 0 for full diff]\n",
					count,
				)
				out[i].IsTruncated = true
				truncatedCount++
			}
		}
	}

	return out, truncatedCount
}

func countDiffLines(hunk string) int {
	trimmed := strings.TrimSuffix(hunk, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func classifyFileRiskTier(f models.FileChange) int {
	if f.IsSensitive {
		return riskTierSensitive
	}
	if isLowPriorityDiffFile(f.Path) {
		return riskTierLow
	}
	return riskTierStandard
}

func isLowPriorityDiffFile(path string) bool {
	norm := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(norm)

	// Documentation files
	for _, ext := range []string{".md", ".mdx", ".txt", ".rst"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	if strings.HasPrefix(norm, "docs/") || strings.HasPrefix(norm, "doc/") || strings.Contains(norm, "/docs/") {
		return true
	}

	// Test files and test directories
	for _, suffix := range []string{
		"_test.go",
		"_test.py",
		".test.ts", ".test.tsx", ".test.js", ".test.jsx",
		".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	if strings.HasPrefix(norm, "test/") || strings.HasPrefix(norm, "tests/") || strings.HasPrefix(norm, "__tests__/") ||
		strings.Contains(norm, "/test/") || strings.Contains(norm, "/tests/") || strings.Contains(norm, "/__tests__/") {
		return true
	}

	return false
}
