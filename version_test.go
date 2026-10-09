package prv

import (
	"os"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	data, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("failed to read VERSION file: %v", err)
	}

	expected := "v" + strings.TrimSpace(string(data))
	if Version() != expected {
		t.Fatalf("expected Version() = %q matching VERSION file, got %q", expected, Version())
	}
}
