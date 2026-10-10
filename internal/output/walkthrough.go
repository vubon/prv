package output

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vubon/prv/internal/models"
)

var (
	// Regex patterns for extracting modified symbols from unified diff hunk headers
	hunkHeaderRegex = regexp.MustCompile(`@@\s+-[0-9,]+\s+\+[0-9,]+\s+@@\s*(.*)`)
	funcHeaderRegex = regexp.MustCompile(`(?:func\s+(?:\([^)]+\)\s+)?|def\s+|fn\s+|function\s+)([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	typeHeaderRegex = regexp.MustCompile(`(?:type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:struct|interface|[A-Za-z_])|class\s+([A-Za-z_][A-Za-z0-9_]*)|pub\s+struct\s+([A-Za-z_][A-Za-z0-9_]*))`)
)

func extractSymbol(text string) (name string, isType bool) {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "//") || strings.HasPrefix(text, "/*") || strings.HasPrefix(text, "*") || strings.HasPrefix(text, "#") {
		return "", false
	}

	if fMatches := funcHeaderRegex.FindStringSubmatch(text); len(fMatches) > 1 && fMatches[1] != "" {
		return fMatches[1], false
	}
	if tMatches := typeHeaderRegex.FindStringSubmatch(text); len(tMatches) > 0 {
		for i := 1; i < len(tMatches); i++ {
			if tMatches[i] != "" {
				return tMatches[i], true
			}
		}
	}
	return "", false
}

// FormatWalkthroughMarkdown generates clean, collapsible <details> HTML blocks
// compatible with GitHub Flavored Markdown and popular previewers.
func FormatWalkthroughMarkdown(walkthroughs []models.FileWalkthrough) string {
	if len(walkthroughs) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, w := range walkthroughs {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		// Header tag composition
		var metaParts []string
		diffStats := fmt.Sprintf("(+%d / -%d)", w.Additions, w.Deletions)
		metaParts = append(metaParts, diffStats)

		if w.IsSensitive {
			metaParts = append(metaParts, "🚨 <b>[SENSITIVE]</b>")
		}

		summaryLine := fmt.Sprintf("<b>%s</b> %s", w.File, strings.Join(metaParts, " "))
		if strings.TrimSpace(w.Topic) != "" {
			summaryLine = fmt.Sprintf("%s — <i>%s</i>", summaryLine, strings.TrimSpace(w.Topic))
		}

		sb.WriteString("<details>\n")
		fmt.Fprintf(&sb, "<summary>%s</summary>\n\n", summaryLine)

		// Changes bullet points
		if len(w.Changes) > 0 {
			for _, change := range w.Changes {
				cleanChange := strings.TrimSpace(change)
				if cleanChange != "" {
					if !strings.HasPrefix(cleanChange, "-") && !strings.HasPrefix(cleanChange, "*") {
						cleanChange = "- " + cleanChange
					}
					sb.WriteString(cleanChange + "\n")
				}
			}
		} else {
			// Fallback bullet if no explicit changes were provided
			if w.Additions > 0 || w.Deletions > 0 {
				fmt.Fprintf(&sb, "- Updated implementation (`+%d` / `-%d` lines).\n", w.Additions, w.Deletions)
			} else {
				sb.WriteString("- File modified.\n")
			}
		}

		sb.WriteString("</details>")
	}

	return sb.String()
}

// GenerateWalkthroughFromFiles analyzes FileChange diff hunks and metadata
// to produce heuristic high-level walkthrough summaries for each modified file.
func GenerateWalkthroughFromFiles(files []models.FileChange) []models.FileWalkthrough {
	var walkthroughs []models.FileWalkthrough

	for _, f := range files {
		if f.IsFiltered {
			continue
		}

		w := models.FileWalkthrough{
			File:        f.Path,
			Additions:   f.Additions,
			Deletions:   f.Deletions,
			IsSensitive: f.IsSensitive,
			Topic:       inferFileTopic(f.Path),
		}

		changes := extractChangesFromDiff(f)
		if len(changes) == 0 {
			if f.Status == "deleted" {
				changes = append(changes, fmt.Sprintf("Deleted file (`-%d` lines removed)", f.Deletions))
			} else if f.Status == "added" || (f.Additions > 0 && f.Deletions == 0) {
				changes = append(changes, fmt.Sprintf("Added new file (`+%d` lines)", f.Additions))
			} else {
				changes = append(changes, fmt.Sprintf("Updated implementation (`+%d` / `-%d` lines)", f.Additions, f.Deletions))
			}
		}

		w.Changes = changes
		walkthroughs = append(walkthroughs, w)
	}

	return walkthroughs
}

// extractChangesFromDiff inspects diff hunks to identify functions, types, and imports changed.
func extractChangesFromDiff(f models.FileChange) []string {
	var changes []string
	seenSymbols := make(map[string]bool)

	switch f.Status {
	case "renamed":
		changes = append(changes, fmt.Sprintf("Renamed from `%s`", f.OldPath))
	case "deleted":
		changes = append(changes, fmt.Sprintf("Deleted file (`-%d` lines removed)", f.Deletions))
		return changes
	case "added":
		changes = append(changes, fmt.Sprintf("Created new file with %d lines", f.Additions))
	}

	if !isCodeFile(f.Path) {
		ext := strings.ToLower(filepath.Ext(f.Path))
		switch ext {
		case ".md", ".txt":
			changes = append(changes, fmt.Sprintf("Updated documentation content (`+%d` / `-%d` lines)", f.Additions, f.Deletions))
		case ".yaml", ".yml", ".json", ".toml", ".ini":
			changes = append(changes, fmt.Sprintf("Updated configuration settings (`+%d` / `-%d` lines)", f.Additions, f.Deletions))
		default:
			changes = append(changes, fmt.Sprintf("Updated file content (`+%d` / `-%d` lines)", f.Additions, f.Deletions))
		}
		return changes
	}

	lines := strings.Split(f.DiffHunk, "\n")
	for _, line := range lines {
		// 1. Inspect hunk headers: @@ -a,b +c,d @@ <context>
		if matches := hunkHeaderRegex.FindStringSubmatch(line); len(matches) > 1 {
			context := strings.TrimSpace(matches[1])
			if sym, isType := extractSymbol(context); sym != "" && !seenSymbols[sym] {
				seenSymbols[sym] = true
				if isType {
					changes = append(changes, fmt.Sprintf("Modified `%s` type definition", sym))
				} else {
					changes = append(changes, fmt.Sprintf("Modified `%s()`", sym))
				}
			}
		}

		// 2. Inspect added/modified function declarations directly in diff additions
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			trimmed := strings.TrimSpace(line[1:])
			if sym, isType := extractSymbol(trimmed); sym != "" && !seenSymbols[sym] {
				seenSymbols[sym] = true
				if isType {
					changes = append(changes, fmt.Sprintf("Added or updated `%s` declaration", sym))
				} else {
					changes = append(changes, fmt.Sprintf("Added or updated `%s()`", sym))
				}
			}
		}
	}

	return changes
}

// inferFileTopic infers a human-friendly topic tag based on directory and filename conventions.
func inferFileTopic(path string) string {
	cleanPath := filepath.ToSlash(path)

	// Check test files
	if strings.HasSuffix(cleanPath, "_test.go") || strings.Contains(cleanPath, "test") {
		return "Test coverage & assertions"
	}

	// Check documentation
	if strings.HasPrefix(cleanPath, "docs/") || strings.HasSuffix(cleanPath, ".md") {
		return "Documentation & guides"
	}

	// Check configuration & CI
	if strings.HasSuffix(cleanPath, ".yaml") || strings.HasSuffix(cleanPath, ".yml") || strings.HasPrefix(cleanPath, ".github/") {
		return "Configuration & workflows"
	}

	// Common package heuristics
	switch {
	case strings.HasPrefix(cleanPath, "cmd/"):
		return "CLI subcommands & entrypoint"
	case strings.Contains(cleanPath, "internal/diagram"):
		return "Architecture & flow diagrams"
	case strings.Contains(cleanPath, "internal/output"):
		return "Review report & formatting"
	case strings.Contains(cleanPath, "internal/git"):
		return "Git & repository operations"
	case strings.Contains(cleanPath, "internal/context"):
		return "Bundle aggregation & context engine"
	case strings.Contains(cleanPath, "internal/config"):
		return "Settings & repository configuration"
	case strings.Contains(cleanPath, "internal/matrix"):
		return "Engineering evaluation matrix"
	case strings.Contains(cleanPath, "internal/models"):
		return "Domain data structures & schema"
	default:
		dir := filepath.Dir(cleanPath)
		if dir != "." && dir != "/" {
			return fmt.Sprintf("Package %s", filepath.Base(dir))
		}
		return "Core implementation"
	}
}

// isCodeFile determines if a file path corresponds to a source code file.
func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".ts", ".js", ".tsx", ".jsx", ".rs", ".java", ".c", ".cpp", ".h", ".hpp", ".rb", ".php", ".cs", ".swift", ".kt":
		return true
	default:
		return false
	}
}
