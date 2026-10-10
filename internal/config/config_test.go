package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != "1" {
		t.Errorf("expected version 1, got %s", cfg.Version)
	}
	if cfg.Review.Persona != "staff-advisory" {
		t.Errorf("expected persona staff-advisory, got %s", cfg.Review.Persona)
	}
	if len(cfg.Review.CustomRules) == 0 {
		t.Error("expected default custom rules to be populated")
	}
	if len(cfg.Review.SensitivePaths) == 0 {
		t.Error("expected default sensitive paths to be populated")
	}
}

func TestLoadConfig_NonExistentFileReturnsDefault(t *testing.T) {
	tempDir := t.TempDir()
	cfg, err := LoadConfig(tempDir, "")
	if err != nil {
		t.Fatalf("expected no error for missing config, got %v", err)
	}
	if cfg.Review.Persona != "staff-advisory" {
		t.Errorf("expected default persona, got %s", cfg.Review.Persona)
	}
}

func TestLoadConfig_CustomYAMLFile(t *testing.T) {
	tempDir := t.TempDir()
	yamlContent := `version: "1"
review:
  persona: "strict-linter"
  sensitive_paths:
    - "payments/**"
  custom_rules:
    - "Never log credit cards"
  ignore_patterns:
    - "*.tmp"
`
	configPath := filepath.Join(tempDir, ".prv.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfg, err := LoadConfig(tempDir, "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Review.Persona != "strict-linter" {
		t.Errorf("expected persona strict-linter, got %s", cfg.Review.Persona)
	}
	if len(cfg.Review.SensitivePaths) != 1 || cfg.Review.SensitivePaths[0] != "payments/**" {
		t.Errorf("unexpected sensitive paths: %v", cfg.Review.SensitivePaths)
	}
	if len(cfg.Review.CustomRules) != 1 || cfg.Review.CustomRules[0] != "Never log credit cards" {
		t.Errorf("unexpected custom rules: %v", cfg.Review.CustomRules)
	}
	if len(cfg.Review.IgnorePatterns) != 1 || cfg.Review.IgnorePatterns[0] != "*.tmp" {
		t.Errorf("unexpected ignore patterns: %v", cfg.Review.IgnorePatterns)
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected bool
	}{
		{"internal/auth/**", "internal/auth/jwt.go", true},
		{"internal/auth/**", "internal/auth/handlers/login.go", true},
		{"internal/auth/**", "internal/context/bundler.go", false},
		{"migrations/**", "migrations/001_init.sql", true},
		{"*secret*", "config_secret_keys.go", true},
		{"*secret*", "models/user.go", false},
		{"*.sql", "db/query.sql", true},
		{"*.sql", "db/query.go", false},
		{"**/auth/*", "pkg/internal/auth/token.go", true},
	}

	for _, tc := range tests {
		got := MatchesPath(tc.pattern, tc.path)
		if got != tc.expected {
			t.Errorf("MatchesPath(%q, %q) = %v; want %v", tc.pattern, tc.path, got, tc.expected)
		}
	}
}

func TestIsSensitivePath(t *testing.T) {
	patterns := []string{
		"internal/auth/**",
		"*secret*",
		"migrations/**",
	}

	if !IsSensitivePath("internal/auth/token.go", patterns) {
		t.Error("expected internal/auth/token.go to be sensitive")
	}
	if !IsSensitivePath("app_secrets.env", patterns) {
		t.Error("expected app_secrets.env to be sensitive")
	}
	if IsSensitivePath("internal/git/git.go", patterns) {
		t.Error("expected internal/git/git.go not to be sensitive")
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, ".prv.yaml")

	if err := WriteDefaultConfig(dest); err != nil {
		t.Fatalf("WriteDefaultConfig failed: %v", err)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected created file at %s, got %v", dest, err)
	}

	// Should error if already exists
	if err := WriteDefaultConfig(dest); err == nil {
		t.Error("expected error writing over existing config file")
	}
}
