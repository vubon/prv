package matrix

import (
	"strings"
	"testing"
)

func TestReviewDimensionsCount(t *testing.T) {
	if len(ReviewDimensions) != 9 {
		t.Fatalf("expected 9 review dimensions, got %d", len(ReviewDimensions))
	}

	names := make(map[string]bool)
	for _, dim := range ReviewDimensions {
		if dim.Name == "" {
			t.Error("dimension name cannot be empty")
		}
		if dim.Description == "" {
			t.Errorf("dimension %s has empty description", dim.Name)
		}
		if len(dim.Checklist) == 0 {
			t.Errorf("dimension %s has empty checklist", dim.Name)
		}
		names[dim.Name] = true
	}

	expectedDimensions := []string{
		"1. Intent & Spec Alignment",
		"2. Logic & Bug Detection",
		"3. Security & Data Safety",
		"4. Performance & Scalability",
		"5. Testing & Verification",
		"6. Clean Code & Maintainability",
		"7. Architecture & System Coupling",
		"8. Observability & Operability",
		"9. Blast Radius & Failure Modes (The Weakest Link)",
	}

	for _, expected := range expectedDimensions {
		if !names[expected] {
			t.Errorf("missing expected dimension %q", expected)
		}
	}
}

func TestGenerateMatrixMarkdown(t *testing.T) {
	md := GenerateMatrixMarkdown()

	if !strings.Contains(md, "# Engineering PR Review Matrix") {
		t.Error("markdown output missing header")
	}

	if !strings.Contains(md, "9. Blast Radius & Failure Modes (The Weakest Link)") {
		t.Error("markdown output missing dimension 9")
	}

	if !strings.Contains(md, "7. Architecture & System Coupling") {
		t.Error("markdown output missing dimension 7")
	}

	if !strings.Contains(md, "8. Observability & Operability") {
		t.Error("markdown output missing dimension 8")
	}
}
