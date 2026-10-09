package matrix

// Dimension represents one core engineering review pillar.
type Dimension struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Checklist   []string `json:"checklist"`
}

// ReviewDimensions contains the standard 6 review pillars.
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
}
