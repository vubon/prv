package matrix

import (
	"fmt"
	"strings"
)

// GenerateMatrixMarkdown renders the review matrix into markdown format.
func GenerateMatrixMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Engineering PR Review Matrix\n\n")

	for _, dim := range ReviewDimensions {
		fmt.Fprintf(&sb, "### %s `[%s]`\n", dim.Name, dim.Severity)
		fmt.Fprintf(&sb, "%s\n\n", dim.Description)
		for _, item := range dim.Checklist {
			fmt.Fprintf(&sb, "- [ ] %s\n", item)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
