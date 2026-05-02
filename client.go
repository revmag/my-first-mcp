package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GitHubClient holds the GitHub API client configuration
type GitHubClient struct {
	baseURL string
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

// Repository represents a GitHub repository
type Repository struct {
	Name string `json:"name"`
}

// NewGitHubClient creates a new GitHub API client with default timeout
func NewGitHubClient(baseURL, apiKey string) *GitHubClient {
	timeout := 15 * time.Second // Default 15 second timeout
	return &GitHubClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetTimeout sets a custom timeout for the client
func (gc *GitHubClient) SetTimeout(timeout time.Duration) {
	gc.timeout = timeout
	gc.client.Timeout = timeout
}

// ListRepos fetches the public repositories for a given GitHub username
func (gc *GitHubClient) ListRepos(ctx context.Context, username string) ([]string, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	url := fmt.Sprintf("%s/users/%s/repos", gc.baseURL, username)

	// Create request with context
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	if gc.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("token %s", gc.apiKey))
	}

	// Send request
	resp, err := gc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repositories: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %s %s", resp.Status, string(body))
	}

	// Parse response
	var repos []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract repo names (limit to 10)
	repoNames := make([]string, 0)
// 	repos = []Repository{
//     {Name: "repo1"},
//     {Name: "repo2"},
//     {Name: "repo3"},
// }
	for i, r := range repos {
		if i >= 10 {
			break
		}
		repoNames = append(repoNames, r.Name)
	}

	return repoNames, nil
}

// // 1. Parse JSON into Repository structs
// var repos []Repository
// json.NewDecoder(resp.Body).Decode(&repos)
// // repos = [{Name: "repo1"}, {Name: "repo2"}, {Name: "repo3"}, ...]

// // 2. Extract just the names
// repoNames := make([]string, 0)  // Start empty

// // 3. Loop through repos (max 10)
// for i, r := range repos {
//     if i >= 10 {
//         break  // Stop at 10
//     }
//     repoNames = append(repoNames, r.Name)  // Add name to list
// }

// // repoNames = ["repo1", "repo2", "repo3", ...]  (up to 10 names)
