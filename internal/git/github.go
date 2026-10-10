package git

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vubon/prv/internal/models"
)

// GitHubClient handles interaction with GitHub via gh CLI or REST API.
type GitHubClient struct {
	Token string
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient() *GitHubClient {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return &GitHubClient{Token: token}
}

// ghViewResponse represents the JSON output from `gh pr view --json ...`.
type ghViewResponse struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
}

// FetchPRViaCLI attempts to fetch PR metadata and diff using the `gh` CLI.
func (c *GitHubClient) FetchPRViaCLI(prOrBranch string, repoPath string) (*models.PRMetadata, string, error) {
	// Verify that the gh CLI is available on the host system
	if _, lookErr := exec.LookPath("gh"); lookErr != nil {
		return nil, "", fmt.Errorf("GitHub CLI 'gh' is not installed or not in $PATH (install via 'brew install gh' or https://cli.github.com)")
	}

	// 1. Fetch metadata JSON via `gh pr view`
	args := []string{"pr", "view", prOrBranch, "--json", "number,title,body,author,baseRefName,headRefName,url,createdAt"}
	cmd := exec.Command("gh", args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			stderrMsg := strings.TrimSpace(string(exitErr.Stderr))
			return nil, "", fmt.Errorf("gh pr view failed: %s (%w)", stderrMsg, err)
		}
		return nil, "", fmt.Errorf("gh pr view failed: %w", err)
	}

	var resp ghViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse gh output: %w", err)
	}

	var createdAt time.Time
	if resp.CreatedAt != "" {
		createdAt, _ = time.Parse(time.RFC3339, resp.CreatedAt)
	}

	meta := &models.PRMetadata{
		Number:      resp.Number,
		Title:       resp.Title,
		Description: resp.Body,
		Author:      resp.Author.Login,
		BaseBranch:  resp.BaseRefName,
		HeadBranch:  resp.HeadRefName,
		URL:         resp.URL,
		CreatedAt:   createdAt,
	}

	// 2. Fetch unified diff via `gh pr diff`
	diffCmd := exec.Command("gh", "pr", "diff", prOrBranch)
	if repoPath != "" {
		diffCmd.Dir = repoPath
	}
	diffOut, err := diffCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			stderrMsg := strings.TrimSpace(string(exitErr.Stderr))
			return meta, "", fmt.Errorf("gh pr diff failed: %s (%w)", stderrMsg, err)
		}
		return meta, "", fmt.Errorf("gh pr diff failed: %w", err)
	}

	return meta, string(diffOut), nil
}

// ParsePRURL extracts owner, repo, and PR number from a URL like https://github.com/owner/repo/pull/42
func ParsePRURL(prURL string) (owner, repo string, number int, err error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return "", "", 0, err
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 4 && parts[2] == "pull" {
		num, err := strconv.Atoi(parts[3])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid PR number in URL: %s", parts[3])
		}
		return parts[0], parts[1], num, nil
	}

	return "", "", 0, fmt.Errorf("unsupported GitHub PR URL format: %s", prURL)
}

// FetchPRViaAPI directly fetches PR metadata and diff via GitHub REST API (Zero-Clone Mode without gh CLI).
func (c *GitHubClient) FetchPRViaAPI(owner, repo string, prNumber int) (*models.PRMetadata, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// 1. Fetch PR Metadata JSON
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("github API request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, "", fmt.Errorf("github API returned HTTP %d: %s", res.StatusCode, string(bodyBytes))
	}

	var prData struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		HTMLURL   string `json:"html_url"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.NewDecoder(res.Body).Decode(&prData); err != nil {
		return nil, "", fmt.Errorf("failed to decode PR metadata: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, prData.CreatedAt)
	meta := &models.PRMetadata{
		Number:      prData.Number,
		Title:       prData.Title,
		Description: prData.Body,
		Author:      prData.User.Login,
		BaseBranch:  prData.Base.Ref,
		HeadBranch:  prData.Head.Ref,
		Repo:        fmt.Sprintf("%s/%s", owner, repo),
		URL:         prData.HTMLURL,
		CreatedAt:   createdAt,
	}

	// 2. Fetch Unified Diff Stream (Zero-Clone)
	diffReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return meta, "", err
	}
	diffReq.Header.Set("Accept", "application/vnd.github.v3.diff")
	if c.Token != "" {
		diffReq.Header.Set("Authorization", "Bearer "+c.Token)
	}

	diffRes, err := client.Do(diffReq)
	if err != nil {
		return meta, "", fmt.Errorf("failed to fetch diff stream: %w", err)
	}
	defer func() { _ = diffRes.Body.Close() }()

	if diffRes.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(diffRes.Body)
		return meta, "", fmt.Errorf("github diff API returned HTTP %d: %s", diffRes.StatusCode, strings.TrimSpace(string(errBody)))
	}

	diffBytes, err := io.ReadAll(diffRes.Body)
	if err != nil {
		return meta, "", fmt.Errorf("failed to read diff stream: %w", err)
	}

	return meta, string(diffBytes), nil
}
