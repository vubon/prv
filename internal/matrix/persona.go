package matrix

import (
	"strings"
)

// Standardized persona identifiers recognized by prv.
const (
	PersonaStaffAdvisory    = "staff-advisory"
	PersonaStrictLinter     = "strict-linter"
	PersonaMentor           = "mentor"
	PersonaStaffAdversarial = "staff-adversarial"
)

// KnownPersonas lists the standardized persona names recognized by prv.
var KnownPersonas = []string{
	PersonaStaffAdvisory,
	PersonaStrictLinter,
	PersonaMentor,
	PersonaStaffAdversarial,
}

// IsKnownPersona reports whether a persona name matches one of the known personas (case-insensitive).
func IsKnownPersona(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	for _, k := range KnownPersonas {
		if strings.EqualFold(k, p) {
			return true
		}
	}
	return false
}

// PersonaInstructions returns the specialized advisory instruction block for an opt-in persona.
// For default and label-only personas, it returns an empty string so existing bundles remain unchanged.
func PersonaInstructions(persona string) string {
	p := strings.TrimSpace(persona)
	if strings.EqualFold(p, PersonaStaffAdversarial) {
		return "> [!IMPORTANT]\n" +
			"> **Adversarial Review Posture (Refute, Don't Confirm):**\n" +
			"> 1. Assume the code contains latent bugs, edge-case regressions, concurrency races, or unhandled failures until proven otherwise. Attempt to disprove correctness through counter-examples.\n" +
			"> 2. Confirm findings before adding them to `issues` or assigning severity. Put open questions and unverified concerns in the report `summary`. Classify a finding as `BLOCKING` only when you provide concrete, reproducible evidence (e.g., exact failure input, trace, or quoting the offending lines) in the `evidence` field (`issues[].evidence`). Put optional improvements in `suggestions` or classify them as `NIT`.\n" +
			"> 3. If prior review findings are included above, evaluate and report their status first before introducing new issues.\n" +
			"> 4. Trace blast radius: verify all affected callers, connected components, and data contracts across system boundaries for failure modes.\n" +
			"> 5. `prv report` checks for evidence content in each `BLOCKING` finding. Verify for yourself that the evidence supports the claim. Missing evidence triggers a warning by default. Strict evidence mode (`--strict-evidence` or `review.require_evidence: true`) rejects the report before saving or posting."
	}
	return ""
}
