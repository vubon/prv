package matrix

// Dimension represents one core engineering review pillar.
type Dimension struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Checklist   []string `json:"checklist"`
}

// ReviewDimensions contains the standard 11 review pillars.
var ReviewDimensions = []Dimension{
	{
		Name:        "1. Intent & Spec Alignment",
		Description: "Verify the code fulfills the PR description, issue requirements, and has no unintended breaking changes.",
		Severity:    "BLOCKING",
		Checklist: []string{
			"Does the code match what is described in the PR summary?",
			"Are there out-of-scope files or accidental leftover debug changes?",
			"Are API/contract changes backwards-compatible or explicitly versioned?",
		},
	},
	{
		Name:        "2. Logic & Bug Detection",
		Description: "Prevent runtime panics, unhandled errors, concurrency race conditions, and boundary flaws.",
		Severity:    "BLOCKING",
		Checklist: []string{
			"Are all nil/null pointer lookups safeguarded before dereferencing?",
			"Are errors properly handled and propagated rather than silently swallowed?",
			"Are boundary conditions, off-by-one loops, and empty collections handled?",
			"Are concurrency mechanisms (goroutines, channels, mutexes, atomic operations) thread-safe?",
		},
	},
	{
		Name:        "3. Security & Data Safety",
		Description: "Ensure OWASP safety, authorization enforcement, and zero exposed credentials.",
		Severity:    "CRITICAL",
		Checklist: []string{
			"Are SQL queries parameterized (no string concatenation)?",
			"Are user inputs, headers, and parameters validated and sanitized?",
			"Are sensitive tokens, credentials, or API keys omitted from code and logs?",
			"Are authentication and authorization checks enforced on new endpoints?",
		},
	},
	{
		Name:        "4. Performance & Scalability",
		Description: "Avoid N+1 queries, unindexed lookups, memory leaks, and inefficient algorithms.",
		Severity:    "MAJOR",
		Checklist: []string{
			"Are database queries inside loops batched to prevent N+1 query traps?",
			"Are unclosed file handles, HTTP response bodies, or database rows properly closed with defer?",
			"Are time complexities optimal (e.g. O(1) map lookups vs O(N) linear array searches)?",
			"Are large in-memory payloads streamed/buffered rather than loaded entirely in RAM?",
		},
	},
	{
		Name:        "5. Testing & Verification",
		Description: "Ensure test coverage for new business logic, regression prevention, and failure cases.",
		Severity:    "MAJOR",
		Checklist: []string{
			"Are new functions and workflows covered with unit or integration tests?",
			"Are error paths and edge cases tested, not just the happy path?",
			"Are mocks appropriate and not masking actual system behavior?",
		},
	},
	{
		Name:        "6. Clean Code & Maintainability",
		Description: "Adhere to project idioms, clean naming, single responsibility, and readability.",
		Severity:    "MINOR",
		Checklist: []string{
			"Are variables, functions, and types named descriptively?",
			"Is the logic clean and easy to understand for future maintainers?",
			"Is non-obvious domain logic clearly commented?",
		},
	},
	{
		Name:        "7. Architecture & System Coupling",
		Description: "Evaluate package boundaries, abstraction leakage, interface cohesion, and long-term tech debt.",
		Severity:    "MAJOR",
		Checklist: []string{
			"Does this change introduce tight coupling or break existing package/layer encapsulation?",
			"Are new abstractions and interfaces minimal, cohesive, and genuinely necessary (avoiding premature abstraction)?",
			"Will this change make future modifications, extensions, or refactoring significantly more difficult?",
			"Does the change respect separation of concerns and single responsibility?",
		},
	},
	{
		Name:        "8. Observability & Operability",
		Description: "Ensure runtime visibility, actionable diagnostics, and operational debuggability when issues arise.",
		Severity:    "MAJOR",
		Checklist: []string{
			"Are error messages descriptive, actionable, and preserved with underlying causes (%w) rather than opaque?",
			"Are critical state transitions, degraded modes, and external failures logged with structured context?",
			"If an external dependency or network call fails, will on-call engineers immediately know why from output/logs?",
			"Are timeouts, cancellation contexts, and resource cleanup properly wired for operational reliability?",
		},
	},
	{
		Name:        "9. Blast Radius & Failure Modes (The Weakest Link)",
		Description: "Stress-test assumptions, identify single points of failure, worst-case degradations, and rollout risks.",
		Severity:    "CRITICAL",
		Checklist: []string{
			"What is the single most fragile part or weakest link in this change, and how will it fail under stress?",
			"What happens if external dependencies (network, API, disk, CLI) are down, slow, or returning invalid payloads?",
			"What is the blast radius of a failure (isolated component vs cascading system-wide outage)?",
			"Is the change safely roll-backable without database/schema corruption or client state incompatibility?",
		},
	},
	{
		Name:        "10. Data Contract & Migration Safety",
		Description: "Prevent schema migration lockups, breaking API serialization drift, and rolling deployment incompatibilities.",
		Severity:    "BLOCKING",
		Checklist: []string{
			"Are database migrations backward-compatible with the previous code version (expand-and-contract pattern)?",
			"Do database migrations avoid long exclusive table locks on large tables (e.g. non-concurrent index creation or table rewrites)?",
			"Are API/gRPC/JSON data contracts backward-compatible with existing clients (no deleted or renamed fields without versioning)?",
			"Are shared cached structures (Redis, memcached) resilient if old and new application instances run simultaneously during rollout?",
		},
	},
	{
		Name:        "11. Environment & Configuration Hygiene",
		Description: "Ensure environment variable completeness, safe boot defaults, documentation sync, and feature flag safety.",
		Severity:    "MAJOR",
		Checklist: []string{
			"Are new required environment variables documented in .env.example, deployment manifests, or configuration templates?",
			"Are safe fallback defaults provided so omitting an optional configuration does not cause runtime panics on boot?",
			"Are feature flags wired with safe fallback behavior and clear deprecation or cleanup plans?",
			"Has user-facing behavior, CLI flags, or public interfaces changed without corresponding documentation updates (README, docs)?",
		},
	},
}
