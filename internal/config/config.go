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
	Version string       `yaml:"version"`
	Review  ReviewConfig `yaml:"review"`
}

// ReviewConfig contains rules, sensitive paths, persona, and strictness.
type ReviewConfig struct {
	Persona        string           `yaml:"persona"` // e.g. "staff-advisory", "strict-linter", "mentor"
	Strictness     StrictnessConfig `yaml:"strictness"`
	SensitivePaths []string         `yaml:"sensitive_paths"` // Directories requiring extra security scrutiny
	CustomRules    []string         `yaml:"custom_rules"`    // Team rules injected into review prompts
	IgnorePatterns []string         `yaml:"ignore_patterns"` // Extra file glob patterns to filter out
}

// StrictnessConfig defines blocking criteria for review verdicts.
type StrictnessConfig struct {
	BlockingDimensions []string `yaml:"blocking_dimensions"`
}

// DefaultConfig returns the default prv configuration when no file exists.
func DefaultConfig() *Config {
	return &Config{
		Version: "1",
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
	return `# .prv.yaml — PR Review Configuration & Team Standards
version: "1"

review:
  # Review persona: "staff-advisory" (default) | "strict-linter" | "mentor"
  persona: "staff-advisory"

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
    - "Exported Go errors must be wrapped with %w"
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
`
}

// WriteDefaultConfig creates a starter .prv.yaml file at the destination path.
func WriteDefaultConfig(destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("configuration file already exists at %s", destPath)
	}
	return os.WriteFile(destPath, []byte(DefaultYAMLContent()), 0644)
}
