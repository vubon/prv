package git

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewGitHubClient(t *testing.T) {
	// Test environment variable detection
	origGH := os.Getenv("GH_TOKEN")
	origGitHub := os.Getenv("GITHUB_TOKEN")
	defer func() {
		_ = os.Setenv("GH_TOKEN", origGH)
		_ = os.Setenv("GITHUB_TOKEN", origGitHub)
	}()

	_ = os.Setenv("GITHUB_TOKEN", "token-from-github")
	client := NewGitHubClient()
	if client.Token != "token-from-github" {
		t.Errorf("expected token 'token-from-github', got %q", client.Token)
	}

	_ = os.Unsetenv("GITHUB_TOKEN")
	_ = os.Setenv("GH_TOKEN", "token-from-gh")
	clientGH := NewGitHubClient()
	if clientGH.Token != "token-from-gh" {
		t.Errorf("expected token 'token-from-gh', got %q", clientGH.Token)
	}
}

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		url        string
		wantOwner  string
		wantRepo   string
		wantNumber int
		wantErr    bool
	}{
		{
			url:        "https://github.com/vubon/prv/pull/42",
			wantOwner:  "vubon",
			wantRepo:   "prv",
			wantNumber: 42,
			wantErr:    false,
		},
		{
			url:        "https://github.com/vubon/prv/pull/5/",
			wantOwner:  "vubon",
			wantRepo:   "prv",
			wantNumber: 5,
			wantErr:    false,
		},
		{
			url:     "https://github.com/vubon/prv/issues/42",
			wantErr: true,
		},
		{
			url:     "https://github.com/vubon/prv/pull/not-a-number",
			wantErr: true,
		},
		{
			url:     "invalid-url-string",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		owner, repo, num, err := ParsePRURL(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParsePRURL(%q) expected error, got nil", tt.url)
			}
			continue
		}

		if err != nil {
			t.Errorf("ParsePRURL(%q) unexpected error: %v", tt.url, err)
			continue
		}

		if owner != tt.wantOwner || repo != tt.wantRepo || num != tt.wantNumber {
			t.Errorf("ParsePRURL(%q) = (%q, %q, %d), want (%q, %q, %d)",
				tt.url, owner, repo, num, tt.wantOwner, tt.wantRepo, tt.wantNumber)
		}
	}
}

func TestParseRepoFromRemoteURL(t *testing.T) {
	tests := []struct {
		remoteURL string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			remoteURL: "git@github.com:vubon/prv.git",
			wantOwner: "vubon",
			wantRepo:  "prv",
			wantErr:   false,
		},
		{
			remoteURL: "git@github.com:vubon/prv",
			wantOwner: "vubon",
			wantRepo:  "prv",
			wantErr:   false,
		},
		{
			remoteURL: "https://github.com/vubon/prv.git",
			wantOwner: "vubon",
			wantRepo:  "prv",
			wantErr:   false,
		},
		{
			remoteURL: "https://github.com/vubon/prv",
			wantOwner: "vubon",
			wantRepo:  "prv",
			wantErr:   false,
		},
		{
			remoteURL: "ssh://git@github.com/vubon/prv.git",
			wantOwner: "vubon",
			wantRepo:  "prv",
			wantErr:   false,
		},
		{
			remoteURL: "invalid-remote",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		owner, repo, err := ParseRepoFromRemoteURL(tt.remoteURL)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRepoFromRemoteURL(%q) expected error, got nil", tt.remoteURL)
			}
			continue
		}

		if err != nil {
			t.Errorf("ParseRepoFromRemoteURL(%q) unexpected error: %v", tt.remoteURL, err)
			continue
		}

		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseRepoFromRemoteURL(%q) = (%q, %q), want (%q, %q)",
				tt.remoteURL, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestPostPRComment_ValidationErrors(t *testing.T) {
	client := NewGitHubClient()

	if err := client.PostPRComment("", "body", "."); err == nil {
		t.Error("expected error for empty PR identifier")
	}

	if err := client.PostPRComment("42", "   ", "."); err == nil {
		t.Error("expected error for whitespace comment body")
	}
}

func TestPostPRCommentViaAPI_MockServer(t *testing.T) {
	var receivedBody string
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedBody = payload["body"]

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 12345, "body": "test comment"}`))
	}))
	defer server.Close()

	client := &GitHubClient{
		Token:   "test-secret-token",
		BaseURL: server.URL,
	}

	err := client.PostPRCommentViaAPI("vubon", "prv", 42, "hello from test")
	if err != nil {
		t.Fatalf("PostPRCommentViaAPI failed: %v", err)
	}

	if authHeader != "Bearer test-secret-token" {
		t.Errorf("expected Bearer test-secret-token, got %q", authHeader)
	}
	if receivedBody != "hello from test" {
		t.Errorf("expected 'hello from test', got %q", receivedBody)
	}

	// Test error status code handling
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message": "Resource not accessible by integration"}`))
	}))
	defer errServer.Close()

	errClient := &GitHubClient{
		Token:   "test-secret-token",
		BaseURL: errServer.URL,
	}
	if err := errClient.PostPRCommentViaAPI("vubon", "prv", 42, "hello"); err == nil {
		t.Error("expected error for HTTP 403 Forbidden, got nil")
	}
}

func TestFetchPRViaAPI_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "diff") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("diff --git a/main.go b/main.go\n+func main() {}"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"number": 42,
			"title": "Mock PR",
			"body": "PR description",
			"user": {"login": "vubon"},
			"base": {"ref": "main"},
			"head": {"ref": "feat/mock"},
			"html_url": "https://github.com/vubon/prv/pull/42",
			"created_at": "2026-10-10T12:00:00Z"
		}`))
	}))
	defer server.Close()

	client := &GitHubClient{
		Token:   "mock-token",
		BaseURL: server.URL,
	}

	meta, diff, err := client.FetchPRViaAPI("vubon", "prv", 42)
	if err != nil {
		t.Fatalf("FetchPRViaAPI failed: %v", err)
	}

	if meta.Number != 42 || meta.Title != "Mock PR" || meta.Author != "vubon" {
		t.Errorf("unexpected meta returned: %+v", meta)
	}
	if !strings.Contains(diff, "diff --git a/main.go b/main.go") {
		t.Errorf("unexpected diff returned: %q", diff)
	}
}
