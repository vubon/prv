package output

import (
	"fmt"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// EvidenceViolation identifies a BLOCKING issue that omitted required supporting evidence.
type EvidenceViolation struct {
	Index int
	File  string
	Line  int
	Title string
}

// FormatLocation returns a concise human-readable location label for the violation.
func (v EvidenceViolation) FormatLocation() string {
	file := strings.TrimSpace(v.File)
	switch {
	case file != "" && v.Line > 0:
		return fmt.Sprintf("%s:%d", file, v.Line)
	case file != "":
		return file
	case strings.TrimSpace(v.Title) != "":
		return fmt.Sprintf("issue #%d (%s)", v.Index+1, strings.TrimSpace(v.Title))
	default:
		return fmt.Sprintf("issue #%d", v.Index+1)
	}
}

// ValidateEvidence returns all BLOCKING issues whose Evidence field is blank after trimming.
// Presence is checked deterministically; semantic truth is not verified.
func ValidateEvidence(res *models.ReviewResult) []EvidenceViolation {
	if res == nil {
		return nil
	}

	var violations []EvidenceViolation
	for i, issue := range res.Issues {
		if !strings.EqualFold(strings.TrimSpace(issue.Severity), models.SeverityBlocking) {
			continue
		}
		if hasConcreteEvidence(issue.Evidence) {
			continue
		}
		violations = append(violations, EvidenceViolation{
			Index: i,
			File:  issue.File,
			Line:  issue.LineStart,
			Title: issue.Title,
		})
	}
	return violations
}

// ApplyEvidencePolicy evaluates BLOCKING issues for evidence without mutating res.
// When strict is true and any BLOCKING issue lacks evidence, it returns the violations and an error.
// When strict is false, it returns any violations with a nil error so callers can warn without mutating verdicts.
func ApplyEvidencePolicy(res *models.ReviewResult, strict bool) ([]EvidenceViolation, error) {
	violations := ValidateEvidence(res)
	if len(violations) == 0 {
		return nil, nil
	}
	if !strict {
		return violations, nil
	}

	locs := formatViolationLocations(violations)
	return violations, fmt.Errorf("%d BLOCKING issue(s) have no evidence (%s); add concrete 'evidence' for each BLOCKING issue or disable strict evidence enforcement", len(violations), locs)
}

// FormatEvidenceWarning formats a user-facing warning message for non-strict runs with missing evidence.
func FormatEvidenceWarning(violations []EvidenceViolation) string {
	if len(violations) == 0 {
		return ""
	}
	locs := formatViolationLocations(violations)
	return fmt.Sprintf("⚠️  %d BLOCKING issue(s) have no evidence (%s).\n    Use --strict-evidence or set review.require_evidence: true to enforce.\n", len(violations), locs)
}

func formatViolationLocations(violations []EvidenceViolation) string {
	locs := make([]string, 0, len(violations))
	for _, v := range violations {
		locs = append(locs, v.FormatLocation())
	}
	return strings.Join(locs, ", ")
}

func formatEvidenceSection(evidence string) string {
	body := trimBlankBoundaryLines(evidence)
	if body == "" {
		return ""
	}
	if isSingleEnclosingFence(body) {
		return "**Evidence:**\n\n" + body + "\n\n"
	}
	return "**Evidence:**\n\n" + MarkdownLiteralBlock(body) + "\n"
}

// MarkdownLiteralBlock wraps text in a fence that cannot be closed by the text itself.
func MarkdownLiteralBlock(text string) string {
	fence := backtickFenceFor(text, 3)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return fence + "text\n" + text + fence + "\n"
}

func hasConcreteEvidence(evidence string) bool {
	body := trimBlankBoundaryLines(evidence)
	if body == "" {
		return false
	}
	lines := strings.Split(body, "\n")
	inFence := false
	var fenceChar byte
	var fenceLen int
	for _, line := range lines {
		if !inFence {
			if fc, fl, ok := parseFenceOpen(line); ok {
				inFence = true
				fenceChar = fc
				fenceLen = fl
				continue
			}
			if strings.TrimSpace(line) != "" {
				return true
			}
			continue
		}
		if isFenceClose(line, fenceChar, fenceLen) {
			inFence = false
			continue
		}
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// trimBlankBoundaryLines strips leading and trailing blank lines (and trailing tabs/spaces on
// the final line) while preserving leading horizontal indentation on every content line.
func trimBlankBoundaryLines(s string) string {
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start == len(lines) {
		return ""
	}
	end := len(lines) - 1
	for end > start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	lines[end] = strings.TrimRight(lines[end], " \t")
	return strings.Join(lines[start:end+1], "\n")
}

func isSingleEnclosingFence(body string) bool {
	lines := strings.Split(body, "\n")
	if len(lines) < 2 {
		return false
	}
	fc, fl, ok := parseFenceOpen(lines[0])
	if !ok {
		return false
	}
	for i := 1; i < len(lines)-1; i++ {
		if isFenceClose(lines[i], fc, fl) {
			return false
		}
	}
	return isFenceClose(lines[len(lines)-1], fc, fl)
}

func parseFenceOpen(line string) (fenceChar byte, fenceLen int, ok bool) {
	spaces := 0
	for spaces < len(line) && spaces < 3 && line[spaces] == ' ' {
		spaces++
	}
	if spaces >= len(line) {
		return 0, 0, false
	}
	ch := line[spaces]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	cnt := leadingCount(line[spaces:], ch)
	if cnt < 3 {
		return 0, 0, false
	}
	rest := line[spaces+cnt:]
	if ch == '`' && strings.ContainsRune(rest, '`') {
		return 0, 0, false
	}
	return ch, cnt, true
}

func isFenceClose(line string, fenceChar byte, minLen int) bool {
	spaces := 0
	for spaces < len(line) && spaces < 3 && line[spaces] == ' ' {
		spaces++
	}
	if spaces >= len(line) || line[spaces] != fenceChar {
		return false
	}
	cnt := leadingCount(line[spaces:], fenceChar)
	if cnt < minLen {
		return false
	}
	return strings.Trim(line[spaces+cnt:], " \t") == ""
}

func leadingCount(s string, ch byte) int {
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return n
}

func backtickFenceFor(content string, minLen int) string {
	maxRun := 0
	cur := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '`' {
			cur++
			if cur > maxRun {
				maxRun = cur
			}
		} else {
			cur = 0
		}
	}
	fenceLen := minLen
	if maxRun >= fenceLen {
		fenceLen = maxRun + 1
	}
	return strings.Repeat("`", fenceLen)
}
