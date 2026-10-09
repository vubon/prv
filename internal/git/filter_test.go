package git

import (
	"testing"
)

func TestShouldFilterFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"package-lock.json", true},
		{"frontend/package-lock.json", true},
		{"go.sum", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"bundle.min.js", true},
		{"static/style.min.css", true},
		{"api/v1/service.pb.go", true},
		{"logo.png", true},
		{"assets/hero.jpg", true},
		{"doc.pdf", true},
		{"node_modules/express/index.js", true},
		{"vendor/github.com/pkg/errors/errors.go", true},
		{"dist/bundle.js", true},
		{"web/dist/app.js", true},
		{"subdist/worker.go", false},
		{"internal/auth/token.go", false},
		{"cmd/server/main.go", false},
		{"frontend/src/App.tsx", false},
		{"pkg/utils/math.go", false},
	}

	for _, tt := range tests {
		filtered, _ := ShouldFilterFile(tt.path)
		if filtered != tt.expected {
			t.Errorf("ShouldFilterFile(%q) = %v; want %v", tt.path, filtered, tt.expected)
		}
	}
}
