package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level .prv.yaml configuration schema.
type Config struct {
	Version string        `yaml:"version"`
	Context ContextConfig `yaml:"context"`
	Review  ReviewConfig  `yaml:"review"`
}

// ContextConfig holds reviewer-provided context directory and safety limits.
type ContextConfig struct {
	Dir       string `yaml:"dir"`         // Designated context directory (default: .prv/context)
	MaxSizeKB int    `yaml:"max_size_kb"` // Maximum allowed size per context file in KB (default: 256; 0 disables limit)
}

// ReviewConfig contains rules, sensitive paths, persona, and strictness.
type ReviewConfig struct {
	Persona           string           `yaml:"persona"`             // e.g. "staff-advisory", "strict-linter", "mentor"
	DefaultBaseBranch string           `yaml:"default_base_branch"` // e.g. "main", "staging", "dev", "develop"
	Strictness        StrictnessConfig `yaml:"strictness"`
	SensitivePaths    []string         `yaml:"sensitive_paths"` // Directories requiring extra security scrutiny
	CustomRules       []string         `yaml:"custom_rules"`    // Team rules injected into review prompts
	IgnorePatterns    []string         `yaml:"ignore_patterns"` // Extra file glob patterns to filter out
}

// StrictnessConfig defines blocking criteria for review verdicts.
type StrictnessConfig struct {
	BlockingDimensions []string `yaml:"blocking_dimensions"`
}

// DefaultConfig returns the default prv configuration when no file exists.
func DefaultConfig() *Config {
	return &Config{
		Version: "1",
		Context: ContextConfig{
			Dir:       ".prv/context",
			MaxSizeKB: 256,
		},
		Review: ReviewConfig{
			Persona: "staff-advisory",
			Strictness: StrictnessConfig{
				BlockingDimensions: []string{"logic", "security", "blast-radius"},
			},
			SensitivePaths: []string{
				"internal/auth/**",
				"auth/**",
				"**/auth/**",
				"migrations/**",
				"db/migrations/**",
				"**/migrations/**",
				"*secret*",
				"*credential*",
			},
			CustomRules:    []string{},
			IgnorePatterns: []string{},
		},
	}
}

// LoadConfig attempts to load .prv.yaml from repoPath or customConfigPath.
// If no file exists, it gracefully returns DefaultConfig() with no error.
func LoadConfig(repoPath string, customConfigPath string) (*Config, error) {
	configPath := customConfigPath

	if configPath == "" {
		candidates := []string{
			filepath.Join(repoPath, ".prv.yaml"),
			filepath.Join(repoPath, ".prv.yml"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}
		}
	}

	// If no config file found, return default configuration gracefully
	if configPath == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) && customConfigPath == "" {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config in %s: %w", configPath, err)
	}

	// Apply sensible fallbacks if empty
	if cfg.Review.Persona == "" {
		cfg.Review.Persona = "staff-advisory"
	}
	if cfg.Context.Dir == "" {
		cfg.Context.Dir = ".prv/context"
	}
	if cfg.Context.MaxSizeKB < 0 {
		cfg.Context.MaxSizeKB = 256
	}

	return cfg, nil
}

// MatchesPath checks if a given file path matches a glob or recursive glob pattern (e.g. "internal/auth/**", "**/*.sql", "**/mocks/**").
func MatchesPath(pattern, path string) bool {
	// Normalize path separators to forward slash
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Clean paths
	pattern = strings.TrimPrefix(pattern, "./")
	path = strings.TrimPrefix(path, "./")

	// Exact match
	if pattern == path {
		return true
	}

	// 1. Double-star prefix and suffix: e.g. "**/mocks/**" or "**/auth/**"
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		mid := strings.TrimPrefix(strings.TrimSuffix(pattern, "/**"), "**/")
		segs := strings.Split(path, "/")
		for i := 0; i < len(segs); i++ {
			if matched, _ := filepath.Match(mid, segs[i]); matched {
				return true
			}
		}
		if path == mid {
			return true
		}
	}

	// 2. Wildcard directory suffix: e.g. "internal/auth/**" or "migrations/**"
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	// 3. Wildcard directory prefix: e.g. "**/*.sql", "**/*_mock.go", "**/auth/*"
	if strings.HasPrefix(pattern, "**/") {
		rest := strings.TrimPrefix(pattern, "**/")
		// Match against base filename if rest has no directory separators
		base := filepath.Base(path)
		if matched, _ := filepath.Match(rest, base); matched {
			return true
		}
		// Match against any trailing path segments across arbitrary directory depth
		segs := strings.Split(path, "/")
		for i := 0; i < len(segs); i++ {
			subpath := strings.Join(segs[i:], "/")
			if matched, _ := filepath.Match(rest, subpath); matched {
				return true
			}
		}
	}

	// 4. Keyword wildcard matching: e.g. "*secret*", "*credential*" anywhere in path
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && strings.Count(pattern, "*") == 2 && !strings.Contains(pattern, "/") {
		keyword := strings.Trim(pattern, "*")
		if strings.Contains(strings.ToLower(path), strings.ToLower(keyword)) {
			return true
		}
	}

	// 5. Standard filepath.Match against full path
	if matched, err := filepath.Match(pattern, path); err == nil && matched {
		return true
	}

	// 6. Standard filepath.Match against base filename (e.g. "*.key", "Makefile*")
	base := filepath.Base(path)
	if matched, err := filepath.Match(pattern, base); err == nil && matched {
		return true
	}

	return false
}

// IsSensitivePath checks if a file path matches any pattern in sensitivePaths list.
func IsSensitivePath(path string, sensitivePaths []string) bool {
	for _, pattern := range sensitivePaths {
		if MatchesPath(pattern, path) {
			return true
		}
	}
	return false
}

// DefaultYAMLContent returns the initial starter .prv.yaml file content.
func DefaultYAMLContent() string {
	return DefaultYAMLContentWithContextDir(".prv/context")
}

// DefaultYAMLContentWithContextDir returns starter .prv.yaml content with a specified context directory.
func DefaultYAMLContentWithContextDir(contextDir string) string {
	if contextDir == "" {
		contextDir = ".prv/context"
	}
	return fmt.Sprintf(`# .prv.yaml — PR Review Configuration & Team Standards
version: "1"

context:
  # Designated directory for auto-discovered PR/branch context files (SPDD specs, Reason Canvases)
  # When reviewing PR #12 or branch 'feat/auth', prv checks this folder for 12.md, pr-12.md, or feat-auth.md
  dir: %q

  # Maximum allowed size per context file in KB (default: 256; set 0 for unlimited)
  max_size_kb: 256

review:
  # Review persona: "staff-advisory" (default) | "strict-linter" | "mentor"
  persona: "staff-advisory"

  # Default base branch to compare against (defaults to main/master if omitted)
  # default_base_branch: "staging"

  # Strictness settings for verdicts
  strictness:
    blocking_dimensions:
      - "logic"
      - "security"
      - "blast-radius"

  # Directories requiring extra scrutiny in reviews
  sensitive_paths:
    - "internal/auth/**"
    - "auth/**"
    - "**/auth/**"
    - "migrations/**"
    - "db/migrations/**"
    - "**/migrations/**"
    - "*secret*"
    - "*credential*"


  # Custom engineering standards injected directly into review prompts
  custom_rules:
    - "All database queries must use parameterized placeholders (no string concatenation)"
    - "External HTTP/RPC calls must carry an explicit context with timeout"
    - "Exported Go errors must be wrapped with %%w"
    - "No raw goroutines; manage concurrency through worker pools or errgroup"

  # Additional file globs to ignore during diff bundling
  # (Note: lockfiles, *.pb.go, *_generated.go, and minified assets are already ignored by default)
  ignore_patterns:
    - "dist/**"
    - "build/**"
    - "vendor/**"
    - "mocks/**"
    - "**/*_mock.go"
    - "docs/swagger/**"
`, contextDir)
}

// StarterTemplateContent returns the starter Reason Canvas template for reviewer-provided context.
func StarterTemplateContent() string {
	return `# Feature Specification & Reason Canvas: [Feature Name]

## 1. Goal & Architectural Intent
- **Core Purpose:** What problem does this change solve?
- **User Impact:** What behavior is expected by the end user or client service?

## 2. Invariants & State Transitions
- What state invariants must *never* be violated under any failure condition?
- Key operations:
  1. Operation A: Pre-conditions -> Post-conditions
  2. Operation B: Edge cases & boundary behavior

## 3. Explicit Non-Goals & Prohibited Changes
- Out of scope files or behavior:
- Deprecations or breaking changes explicitly disallowed:

## 4. Security, Data & Migration Contracts
- Authorization & validation requirements:
- Database migration safety (no exclusive table locking, expand-and-contract):
- Environment variables or secrets required:

## 5. Verification & Acceptance Criteria
- [ ] Unit tests covering edge cases
- [ ] Concurrency/race condition safety
- [ ] Backward-compatible API responses
`
}

// WriteDefaultConfig creates a starter .prv.yaml file at the destination path and scaffolds default context directory.
func WriteDefaultConfig(destPath string) error {
	return WriteDefaultConfigWithContextDir(destPath, ".prv/context")
}

// WriteDefaultConfigWithContextDir creates a starter .prv.yaml file with custom context dir and scaffolds template.md.
func WriteDefaultConfigWithContextDir(destPath, contextDir string) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("configuration file already exists at %s", destPath)
	}

	if contextDir == "" {
		contextDir = ".prv/context"
	}

	if filepath.IsAbs(contextDir) || !filepath.IsLocal(filepath.FromSlash(contextDir)) {
		return fmt.Errorf("context directory %q must be a relative path inside the repository", contextDir)
	}

	content := DefaultYAMLContentWithContextDir(contextDir)
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return err
	}

	// Scaffold the context directory and template.md
	baseDir := filepath.Dir(destPath)
	targetContextDir := filepath.Join(baseDir, filepath.FromSlash(contextDir))
	if err := os.MkdirAll(targetContextDir, 0755); err != nil {
		return fmt.Errorf("created %s but failed to create context directory %s: %w", destPath, targetContextDir, err)
	}

	templatePath := filepath.Join(targetContextDir, "template.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		if err := os.WriteFile(templatePath, []byte(StarterTemplateContent()), 0644); err != nil {
			return fmt.Errorf("created %s but failed to create starter template %s: %w", destPath, templatePath, err)
		}
	}

	return nil
}
