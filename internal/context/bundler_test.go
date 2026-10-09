package context

import (
	"testing"

	"github.com/vubon/prv/internal/models"
)

func TestPrepareBundle_InvalidRemotePRReturnsError(t *testing.T) {
	bundler := NewBundler(".")

	// An invalid PR identifier must return an error and not fall back to local branch review
	opts := models.BundleOptions{
		PRIdentifier: "invalid-pr-identifier-that-does-not-exist",
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when fetching invalid remote PR, got nil")
	}
}

func TestPrepareBundle_InvalidPRURLReturnsError(t *testing.T) {
	bundler := NewBundler(".")

	opts := models.BundleOptions{
		PRIdentifier: "https://github.com/invalid-org-12345/non-existent-repo/pull/999999",
	}

	_, err := bundler.PrepareBundle(opts)
	if err == nil {
		t.Fatal("expected error when fetching non-existent PR URL, got nil")
	}
}
