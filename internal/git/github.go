package git

import (
	"bytes"
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
	Token   string
	BaseURL string // Overrides API base URL (defaults to https://api.github.com if empty)
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient() *GitHubClient {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return &GitHubClient{Token: token}
}

func (c *GitHubClient) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
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
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL(), owner, repo, prNumber)
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

// ParseRepoFromRemoteURL extracts owner and repo from an SSH or HTTPS git remote URL.
func ParseRepoFromRemoteURL(remoteURL string) (owner, repo string, err error) {
	trimmed := strings.TrimSpace(remoteURL)
	trimmed = strings.TrimSuffix(trimmed, ".git")

	// SSH style: git@github.com:owner/repo
	if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") && !strings.HasPrefix(trimmed, "ssh://") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			slugParts := strings.Split(parts[1], "/")
			if len(slugParts) == 2 && slugParts[0] != "" && slugParts[1] != "" {
				return slugParts[0], slugParts[1], nil
			}
		}
	}

	// URL style: https://github.com/owner/repo or ssh://git@github.com/owner/repo
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1], nil
	}

	return "", "", fmt.Errorf("cannot parse owner and repo from remote URL: %s", remoteURL)
}

// ResolveRepoAndPR extracts owner, repo, and PR number from a PR URL or integer string,
// inferring owner and repo from the local git remote if a plain number is given.
func (c *GitHubClient) ResolveRepoAndPR(prOrURL string, repoPath string) (owner, repo string, number int, err error) {
	trimmed := strings.TrimSpace(prOrURL)
	if trimmed == "" {
		return "", "", 0, fmt.Errorf("PR identifier (number or URL) cannot be empty")
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return ParsePRURL(trimmed)
	}

	number, numErr := strconv.Atoi(trimmed)
	if numErr != nil {
		return "", "", 0, fmt.Errorf("invalid PR identifier: %s", prOrURL)
	}

	if repoPath != "" {
		runner := NewGitRunner(repoPath)
		if remoteURL, rErr := runner.GetRemoteURL(); rErr == nil {
			if o, r, pErr := ParseRepoFromRemoteURL(remoteURL); pErr == nil {
				return o, r, number, nil
			}
		}
	}

	return "", "", number, fmt.Errorf("cannot infer repository owner and repo from git remote for PR %d (try passing full PR URL)", number)
}

// PostPRComment posts a review comment to a pull request using either gh CLI or the REST API.
func (c *GitHubClient) PostPRComment(prOrURL string, commentBody string, repoPath string) error {
	if strings.TrimSpace(commentBody) == "" {
		return fmt.Errorf("comment body cannot be empty")
	}
	if strings.TrimSpace(prOrURL) == "" {
		return fmt.Errorf("PR identifier (number or URL) cannot be empty")
	}

	// Try gh CLI first if available
	if _, err := exec.LookPath("gh"); err == nil {
		if cliErr := c.PostPRCommentViaCLI(prOrURL, commentBody, repoPath); cliErr == nil {
			return nil
		} else if c.Token == "" {
			// CLI failed and no token configured for API fallback
			return cliErr
		}
	}

	// Fallback to GitHub REST API if GITHUB_TOKEN is available
	if c.Token != "" {
		owner, repo, number, err := c.ResolveRepoAndPR(prOrURL, repoPath)
		if err != nil {
			return err
		}
		return c.PostPRCommentViaAPI(owner, repo, number, commentBody)
	}

	return fmt.Errorf("failed to post PR comment: neither GitHub CLI 'gh' succeeded nor valid GITHUB_TOKEN was available")
}

// PostPRCommentViaCLI publishes a comment using `gh pr comment <pr> --body-file -`.
func (c *GitHubClient) PostPRCommentViaCLI(prOrURL string, commentBody string, repoPath string) error {
	if _, lookErr := exec.LookPath("gh"); lookErr != nil {
		return fmt.Errorf("GitHub CLI 'gh' is not installed or not in $PATH")
	}

	cmd := exec.Command("gh", "pr", "comment", prOrURL, "--body-file", "-")
	if repoPath != "" {
		cmd.Dir = repoPath
	}
	cmd.Stdin = strings.NewReader(commentBody)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr comment failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PostPRCommentViaAPI publishes a comment directly via the GitHub REST API.
func (c *GitHubClient) PostPRCommentViaAPI(owner, repo string, prNumber int, commentBody string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL(), owner, repo, prNumber)

	payload := map[string]string{"body": commentBody}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github comment API request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusCreated {
		errBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("github comment API returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(errBody)))
	}
	return nil
}

// PostPRReview posts a Pull Request Review (with optional inline comments) using either gh CLI or the REST API.
func (c *GitHubClient) PostPRReview(prOrURL string, payload *models.PullRequestReviewPayload, repoPath string) error {
	if payload == nil {
		return fmt.Errorf("review payload cannot be nil")
	}
	if strings.TrimSpace(payload.Body) == "" && len(payload.Comments) == 0 {
		return fmt.Errorf("review payload must contain either a body or inline comments")
	}

	owner, repo, number, err := c.ResolveRepoAndPR(prOrURL, repoPath)
	if err != nil {
		return err
	}

	// Try gh CLI first if available
	if _, lookErr := exec.LookPath("gh"); lookErr == nil {
		if cliErr := c.PostPRReviewViaCLI(owner, repo, number, payload, repoPath); cliErr == nil {
			return nil
		} else if len(payload.Comments) > 0 && strings.Contains(strings.ToLower(cliErr.Error()), "diff") {
			fmt.Fprintf(os.Stderr, "Warning: inline diff anchoring failed (%v); falling back to top-level review\n", cliErr)
			fallbackPayload := *payload
			fallbackPayload.Comments = nil
			if retryErr := c.PostPRReviewViaCLI(owner, repo, number, &fallbackPayload, repoPath); retryErr == nil {
				return nil
			}
		} else if c.Token == "" {
			return cliErr
		}
	}

	// Fallback to GitHub REST API if GITHUB_TOKEN is available
	if c.Token != "" {
		apiErr := c.PostPRReviewViaAPI(owner, repo, number, payload)
		if apiErr == nil {
			return nil
		}
		if len(payload.Comments) > 0 && strings.Contains(strings.ToLower(apiErr.Error()), "422") {
			fmt.Fprintf(os.Stderr, "Warning: inline diff anchoring failed (HTTP 422); falling back to top-level review\n")
			fallbackPayload := *payload
			fallbackPayload.Comments = nil
			if retryErr := c.PostPRReviewViaAPI(owner, repo, number, &fallbackPayload); retryErr == nil {
				return nil
			}
		}
		return apiErr
	}

	return fmt.Errorf("failed to post PR review: neither GitHub CLI 'gh' succeeded nor valid GITHUB_TOKEN was available")
}

// PostPRReviewViaCLI publishes a PR review with inline comments using `gh api repos/<owner>/<repo>/pulls/<number>/reviews --input -`.
func (c *GitHubClient) PostPRReviewViaCLI(owner, repo string, prNumber int, payload *models.PullRequestReviewPayload, repoPath string) error {
	if _, lookErr := exec.LookPath("gh"); lookErr != nil {
		return fmt.Errorf("GitHub CLI 'gh' is not installed or not in $PATH")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal review payload: %w", err)
	}

	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)
	cmd := exec.Command("gh", "api", endpoint, "--input", "-")
	if repoPath != "" {
		cmd.Dir = repoPath
	}
	cmd.Stdin = bytes.NewReader(payloadBytes)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh api PR review failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PostPRReviewViaAPI publishes a PR review with inline comments directly via the GitHub REST API.
func (c *GitHubClient) PostPRReviewViaAPI(owner, repo string, prNumber int, payload *models.PullRequestReviewPayload) error {
	client := &http.Client{Timeout: 30 * time.Second}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", c.baseURL(), owner, repo, prNumber)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal review payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github review API request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		errBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("github review API returned HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(errBody)))
	}
	return nil
}
