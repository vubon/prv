package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IgnoredFilePatterns contains default patterns of files to exclude from AI review context to save tokens and avoid noise.
var IgnoredFilePatterns = []string{
	// Lockfiles
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"Cargo.lock",
	"composer.lock",
	"Gemfile.lock",
	"poetry.lock",
	"mix.lock",

	// Minified & Bundled Assets
	"*.min.js",
	"*.min.css",
	"*.bundle.js",
	"*.map",

	// Compiled / Generated Artifacts
	"*.pb.go",
	"*_generated.go",
	"*.gen.go",
	"schema.graphql.go",

	// Binary & Media
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.ico",
	"*.svg",
	"*.pdf",
	"*.wasm",
	"*.exe",
	"*.dylib",
	"*.so",

	// IDE & OS
	".DS_Store",
	"Thumbs.db",
}

// ShouldFilterFile checks if a file path matches any noise/lockfile pattern.
func ShouldFilterFile(filePath string) (bool, string) {
	base := filepath.Base(filePath)
	ext := filepath.Ext(filePath)

	for _, pattern := range IgnoredFilePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true, "Matches noise pattern: " + pattern
		}
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true, "Matches noise pattern: " + pattern
		}
	}

	// Check common directories with boundary checks
	cleanPath := filepath.ToSlash(filePath)
	ignoredDirs := []string{"node_modules", "vendor", "dist", "build", ".next"}
	for _, dir := range ignoredDirs {
		if strings.HasPrefix(cleanPath, dir+"/") || strings.Contains(cleanPath, "/"+dir+"/") {
			return true, fmt.Sprintf("Inside %s directory", dir)
		}
	}

	// Binary extension checks
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".pdf", ".zip", ".tar", ".gz", ".wasm", ".bin":
		return true, "Binary or media asset"
	}

	return false, ""
}

// ShouldFilterFileWithExtra checks default noise patterns plus any extra ignore patterns from .prv.yaml.
func ShouldFilterFileWithExtra(filePath string, extraPatterns []string) (bool, string) {
	if filtered, reason := ShouldFilterFile(filePath); filtered {
		return true, reason
	}

	base := filepath.Base(filePath)
	clean := filepath.ToSlash(filePath)

	for _, pattern := range extraPatterns {
		pattern = filepath.ToSlash(pattern)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true, "Matches custom ignore pattern: " + pattern
		}
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true, "Matches custom ignore pattern: " + pattern
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
				return true, "Matches custom ignore pattern: " + pattern
			}
		}
	}

	return false, ""
}
