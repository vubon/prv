package matrix

import (
	"strings"
	"testing"
)

func TestIsKnownPersona(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"staff-advisory", true},
		{"Staff-Advisory", true},
		{"STAFF-ADVERSARIAL", true},
		{"staff-adversarial", true},
		{"strict-linter", true},
		{"mentor", true},
		{"Mentor", true},
		{"unknown-persona", false},
		{"", false},
		{"   ", false},
		{"staff_advisory", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := IsKnownPersona(tc.input)
			if got != tc.want {
				t.Errorf("IsKnownPersona(%q) = %v; want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestPersonaInstructions(t *testing.T) {
	// Only staff-adversarial should return an instruction block
	adv := PersonaInstructions(PersonaStaffAdversarial)
	if adv == "" {
		t.Fatalf("expected non-empty instructions for %s, got empty", PersonaStaffAdversarial)
	}
	if !strings.Contains(adv, "Adversarial Review Posture (Refute, Don't Confirm)") {
		t.Errorf("expected adversarial posture heading, got:\n%s", adv)
	}
	if !strings.Contains(adv, "Classify a finding as `BLOCKING` only when you provide concrete, reproducible evidence") {
		t.Errorf("expected evidence requirement in instructions, got:\n%s", adv)
	}
	if !strings.Contains(adv, "issues[].evidence") {
		t.Errorf("expected explicit issues[].evidence field reference in instructions, got:\n%s", adv)
	}
	if !strings.Contains(adv, "Trace blast radius") {
		t.Errorf("expected blast radius step in instructions, got:\n%s", adv)
	}
	if !strings.Contains(adv, "prv report") {
		t.Errorf("expected reference to prv report validation, got:\n%s", adv)
	}

	for _, want := range []string{
		"Confirm findings before adding them to `issues` or assigning severity",
		"Put open questions and unverified concerns in the report `summary`",
		"checks for evidence content",
		"Verify for yourself that the evidence supports the claim",
		"Missing evidence triggers a warning by default",
		"Strict evidence mode (`--strict-evidence` or `review.require_evidence: true`) rejects the report",
	} {
		if !strings.Contains(adv, want) {
			t.Errorf("expected instructions to contain %q, got:\n%s", want, adv)
		}
	}

	// Case-insensitivity and fold consistency check
	if got := PersonaInstructions("Staff-Adversarial"); got != adv {
		t.Errorf("expected case-insensitive matching for staff-adversarial")
	}
	if got := PersonaInstructions("ſtaff-adversarial"); got != adv {
		t.Errorf("expected EqualFold consistency for staff-adversarial")
	}

	// Default, label-only, and unknown personas must return empty string (zero regression)
	for _, p := range []string{
		PersonaStaffAdvisory,
		PersonaStrictLinter,
		PersonaMentor,
		"custom-label",
		"unknown",
		"",
	} {
		if got := PersonaInstructions(p); got != "" {
			t.Errorf("expected empty instructions for %q, got: %q", p, got)
		}
	}
}
