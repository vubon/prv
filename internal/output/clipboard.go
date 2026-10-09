package output

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vubon/prv/internal/models"
)

// CopyToClipboard copies a string directly into the system clipboard.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip or xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("neither xclip nor xsel found for clipboard support")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	cmd.Stdin = bytes.NewReader([]byte(text))
	return cmd.Run()
}

// CopyCommentsToClipboard formats all comments and copies them into the clipboard.
func CopyCommentsToClipboard(res *models.ReviewResult) (int, error) {
	comments := GenerateCommentList(res)
	if len(comments) == 0 {
		return 0, nil
	}

	var sb strings.Builder
	for _, c := range comments {
		sb.WriteString(fmt.Sprintf("<!-- File: %s#L%d -->\n", c.File, c.Line))
		sb.WriteString(c.MarkdownBody)
		sb.WriteString("\n\n---\n\n")
	}

	err := CopyToClipboard(sb.String())
	return len(comments), err
}
