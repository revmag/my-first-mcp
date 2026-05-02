package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

type GitHubClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type RepoDetail struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	Stars         int    `json:"stargazers_count"`
	Forks         int    `json:"forks_count"`
	OpenIssues    int    `json:"open_issues_count"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	UpdatedAt     string `json:"updated_at"`
}

type UserProfile struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	Bio         string `json:"bio"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	HTMLURL     string `json:"html_url"`
	CreatedAt   string `json:"created_at"`
}

type UserSummary struct {
	Login          string
	TotalRepos     int
	TotalStars     int
	TopLanguages   []string
	LanguageCounts map[string]int
}

func NewGitHubClient(baseURL, apiKey string) *GitHubClient {
	return &GitHubClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (gc *GitHubClient) do(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("network error: failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if gc.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", gc.apiKey))
	}

	resp, err := gc.client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: could not reach GitHub API. Check your internet connection or try again later.")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		switch resp.StatusCode {
		case 404:
			return fmt.Errorf("not found: the resource you requested does not exist. Verify the owner and repository name are spelled correctly (e.g., 'octocat/Hello-World'). Use list_repos to discover valid repository names.")
		case 401:
			return fmt.Errorf("authentication failed: invalid or expired API credentials. If using a personal access token, verify it has the correct scopes (repo, public_repo).")
		case 403:
			return fmt.Errorf("access denied: you may have exceeded GitHub's rate limit (60 requests/hour unauthenticated, 5000/hour authenticated). Wait before retrying, or provide a GitHub personal access token for higher limits.")
		case 422:
			return fmt.Errorf("invalid request: the parameters or query format is incorrect. Check that usernames, repo names, and search syntax are valid.")
		default:
			return fmt.Errorf("github api error (HTTP %d): %s. Check the GitHub API documentation if the error persists.", resp.StatusCode, string(body))
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("parsing error: the GitHub API returned unexpected data. This may be a temporary issue. Try again in a few seconds.")
	}
	return nil
}

// ListRepos returns the names of up to 10 public repos for a user.
func (gc *GitHubClient) ListRepos(ctx context.Context, username string) ([]string, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	url := fmt.Sprintf("%s/users/%s/repos?per_page=10", gc.baseURL, username)
	var repos []RepoDetail
	if err := gc.do(ctx, url, &repos); err != nil {
		return nil, err
	}
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	return names, nil
}

// GetRepo returns full metadata for a single repo by owner/name.
func (gc *GitHubClient) GetRepo(ctx context.Context, owner, repo string) (*RepoDetail, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	url := fmt.Sprintf("%s/repos/%s/%s", gc.baseURL, owner, repo)
	var detail RepoDetail
	if err := gc.do(ctx, url, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// SearchRepos searches GitHub repos by keyword, with optional language filter and sort order.
func (gc *GitHubClient) SearchRepos(ctx context.Context, query, language, sortBy string, limit int) ([]RepoDetail, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	if sortBy == "" {
		sortBy = "stars"
	}
	q := query
	if language != "" {
		q += fmt.Sprintf("+language:%s", language)
	}
	url := fmt.Sprintf("%s/search/repositories?q=%s&sort=%s&per_page=%d", gc.baseURL, q, sortBy, limit)

	var result struct {
		Items []RepoDetail `json:"items"`
	}
	if err := gc.do(ctx, url, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetUserProfile returns public profile info for a GitHub user.
func (gc *GitHubClient) GetUserProfile(ctx context.Context, username string) (*UserProfile, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	url := fmt.Sprintf("%s/users/%s", gc.baseURL, username)
	var profile UserProfile
	if err := gc.do(ctx, url, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SummarizeUser fetches up to 100 repos and aggregates star count and language breakdown.
func (gc *GitHubClient) SummarizeUser(ctx context.Context, username string) (*UserSummary, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	url := fmt.Sprintf("%s/users/%s/repos?per_page=100", gc.baseURL, username)
	var repos []RepoDetail
	if err := gc.do(ctx, url, &repos); err != nil {
		return nil, err
	}

	summary := &UserSummary{
		Login:          username,
		TotalRepos:     len(repos),
		LanguageCounts: make(map[string]int),
	}
	for _, r := range repos {
		summary.TotalStars += r.Stars
		if r.Language != "" {
			summary.LanguageCounts[r.Language]++
		}
	}

	type langCount struct {
		lang  string
		count int
	}
	var langs []langCount
	for l, c := range summary.LanguageCounts {
		langs = append(langs, langCount{l, c})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].count > langs[j].count
	})
	for _, lc := range langs {
		summary.TopLanguages = append(summary.TopLanguages, fmt.Sprintf("%s (%d repos)", lc.lang, lc.count))
	}

	return summary, nil
}
