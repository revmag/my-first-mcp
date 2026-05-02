package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var githubClient *GitHubClient

// optString returns the value of an optional string parameter, or "" if absent.
func optString(req mcp.CallToolRequest, key string) string {
	return req.GetString(key, "")
}

// optInt returns the value of an optional int parameter, or def if absent.
func optInt(req mcp.CallToolRequest, key string, def int) int {
	return req.GetInt(key, def)
}

// list_repos: returns up to 10 public repo names for a user.
func listRepos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	username, err := req.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	repos, err := githubClient.ListRepos(ctx, username)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(repos) == 0 {
		return mcp.NewToolResultText("no repos found"), nil
	}

	return mcp.NewToolResultText(strings.Join(repos, "\n")), nil
}

// get_repo: returns full metadata for a single repo by owner and name.
func getRepo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	owner, err := req.RequireString("owner")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter 'owner' (GitHub username). Example: 'torvalds', 'octocat'. First call list_repos(username) if you need to find valid repository names."), nil
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter 'repo' (repository name). Example: 'linux', 'Hello-World'. First call list_repos(owner) if you need to find valid repository names."), nil
	}

	detail, err := githubClient.GetRepo(ctx, owner, repo)
	if err != nil {
		// Check if it's a "not found" error the AI can recover from
		if strings.Contains(err.Error(), "not found") {
			return mcp.NewToolResultError(fmt.Sprintf("Repository '%s/%s' does not exist. Recovery steps: (1) Call list_repos('%s') to find valid repositories, or (2) Check that the owner and repo names are spelled correctly.", owner, repo, owner)), nil
		}
		// For other errors (network, auth, rate limit), provide context
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch repository details: %s", err.Error())), nil
	}

	desc := detail.Description
	if desc == "" {
		desc = "(no description)"
	}
	lang := detail.Language
	if lang == "" {
		lang = "(unknown)"
	}
	out := fmt.Sprintf(
		"Name: %s\nFull Name: %s\nDescription: %s\nLanguage: %s\nStars: %d\nForks: %d\nOpen Issues: %d\nDefault Branch: %s\nURL: %s\nLast Updated: %s",
		detail.Name, detail.FullName, desc, lang,
		detail.Stars, detail.Forks, detail.OpenIssues,
		detail.DefaultBranch, detail.HTMLURL, detail.UpdatedAt,
	)
	return mcp.NewToolResultText(out), nil
}

// search_repos: searches GitHub repos by keyword with optional language filter and sort.
func searchRepos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter 'query' (search keywords). Examples: 'kubernetes', 'http server', 'machine learning'. Be specific — broad queries like 'code' may not return useful results."), nil
	}

	language := optString(req, "language")
	sortBy := optString(req, "sort")
	limit := optInt(req, "limit", 10)

	results, err := githubClient.SearchRepos(ctx, query, language, sortBy, limit)
	if err != nil {
		// Distinguish recoverable vs non-recoverable errors
		if strings.Contains(err.Error(), "rate limit") {
			return mcp.NewToolResultError(fmt.Sprintf("Rate limit exceeded: %s. Recovery: Wait a few minutes before retrying, or provide a GitHub personal access token for higher limits.", err.Error())), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %s", err.Error())), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("no repositories found"), nil
	}

	var lines []string
	for _, r := range results {
		desc := r.Description
		if desc == "" {
			desc = "(no description)"
		}
		lines = append(lines, fmt.Sprintf("%s — %s [%s] ★%d", r.FullName, desc, r.Language, r.Stars))
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// get_user_profile: returns a GitHub user's public profile.
func getUserProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	username, err := req.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter 'username' (GitHub login). Example: 'torvalds', 'octocat'. This should be the user's login name, not their display name."), nil
	}

	p, err := githubClient.GetUserProfile(ctx, username)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return mcp.NewToolResultError(fmt.Sprintf("User '%s' does not exist on GitHub. Verify the username is spelled correctly and is a valid GitHub account (case-sensitive).", username)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch user profile: %s", err.Error())), nil
	}

	name := p.Name
	if name == "" {
		name = "(no display name)"
	}
	bio := p.Bio
	if bio == "" {
		bio = "(no bio)"
	}
	out := fmt.Sprintf(
		"Login: %s\nName: %s\nBio: %s\nCompany: %s\nLocation: %s\nFollowers: %d\nFollowing: %d\nPublic Repos: %d\nURL: %s\nMember Since: %s",
		p.Login, name, bio, p.Company, p.Location,
		p.Followers, p.Following, p.PublicRepos,
		p.HTMLURL, p.CreatedAt,
	)
	return mcp.NewToolResultText(out), nil
}

// summarize_user: aggregates stats across all public repos for a user.
func summarizeUser(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	username, err := req.RequireString("username")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter 'username' (GitHub login). Example: 'torvalds', 'octocat'."), nil
	}

	s, err := githubClient.SummarizeUser(ctx, username)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return mcp.NewToolResultError(fmt.Sprintf("User '%s' does not exist. This error may also occur if the user has no public repositories. Verify the username and try again.", username)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Failed to summarize user: %s", err.Error())), nil
	}

	langs := "(none)"
	if len(s.TopLanguages) > 0 {
		langs = strings.Join(s.TopLanguages, ", ")
	}
	out := fmt.Sprintf(
		"User: %s\nTotal Public Repos: %d\nTotal Stars: %d\nLanguages: %s",
		s.Login, s.TotalRepos, s.TotalStars, langs,
	)
	return mcp.NewToolResultText(out), nil
}

func main() {
	githubClient = NewGitHubClient("https://api.github.com", "")

	s := server.NewMCPServer("github-mcp", "1.0.0")

	// Server-wide usage rules
	const serverRules = `
## GitHub MCP Server — Usage Rules

**Before you call any tool:**
1. Always start with list_repos or get_user_profile to verify the username exists
2. Use get_repo or summarize_user only after confirming the user exists
3. search_repos is discovery — use get_repo for deep details on known repos

**Error handling:**
- "not found" errors are recoverable: call list_repos to find valid names
- "rate limit" errors are recoverable: wait a few minutes and retry
- "network error" or "authentication failed" are temporary: retry after a short delay
- Do NOT retry with identical parameters if you get the same error twice

**Avoiding infinite loops:**
- If a call fails with "not found", switch to list_repos or search_repos — don't retry get_repo with the same parameters
- If a call fails with "rate limit", wait and try a different query or tool
- If a call fails twice identically, report the error to the user instead of looping
`
	_ = serverRules // TODO: make this available to clients

	s.AddTool(
		mcp.NewTool("list_repos",
			mcp.WithDescription("List up to 10 public repository names for a GitHub user. Use this for a quick overview of what repos a user has. Returns only names — use get_repo for full details on a specific repo. USAGE RULE: Always call this before get_repo if you don't know the exact repo name."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds', 'octocat')")),
		),
		listRepos,
	)

	s.AddTool(
		mcp.NewTool("get_repo",
			mcp.WithDescription("Fetch full metadata for a single GitHub repository. Use when you know the exact owner and repo name and need details like star count, forks, open issues, primary language, description, default branch, or last updated time. USAGE RULE: Only call this if you've already verified the repo exists via list_repos or search_repos. If you get a 'not found' error, call list_repos to find the correct repo name."),
			mcp.WithString("owner", mcp.Required(), mcp.Description("GitHub login of the repo owner (e.g. 'torvalds')")),
			mcp.WithString("repo", mcp.Required(), mcp.Description("Exact repository name (e.g. 'linux'). Case-sensitive. If you're unsure, call list_repos(owner) first.")),
		),
		getRepo,
	)

	s.AddTool(
		mcp.NewTool("search_repos",
			mcp.WithDescription("Search GitHub repositories by keyword for discovery. Use when the user wants to find repos matching a topic or technology, or when you don't know the exact repo name. Supports optional language filter and sort by 'stars', 'forks', or 'updated'. Example: search for 'kubernetes operator' in Go sorted by stars. After discovering a repo, use get_repo for full details."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search keywords (e.g. 'http server', 'machine learning framework'). Be specific — avoid single words like 'code' or 'app' which return too many results.")),
			mcp.WithString("language", mcp.Description("Filter by primary programming language (e.g. 'go', 'python', 'typescript', 'javascript'). Omit to search all languages.")),
			mcp.WithString("sort", mcp.Description("Sort order: 'stars' (default, most popular first), 'forks' (most forked), or 'updated' (recently changed).")),
			mcp.WithNumber("limit", mcp.Description("Number of results to return (1-20, default 10). Lower limits are faster.")),
		),
		searchRepos,
	)

	s.AddTool(
		mcp.NewTool("get_user_profile",
			mcp.WithDescription("Fetch a GitHub user's public profile (name, bio, location, followers, repo count, etc). Use this to answer 'who is X' questions or learn about a developer's identity. USAGE RULE: Call this before list_repos to verify the user exists. If you get a 'not found' error, the user doesn't exist on GitHub."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds', 'octocat'). This is the user's username, not their display name. Case-sensitive.")),
		),
		getUserProfile,
	)

	s.AddTool(
		mcp.NewTool("summarize_user",
			mcp.WithDescription("Aggregate high-level statistics across a user's public repos: total repos, total stars across all repos, and language breakdown. Use when the user asks for an overview of a developer's presence or tech stack. This tool fetches up to 100 repos, so it's slower but more comprehensive than list_repos. USAGE RULE: Call get_user_profile first to verify the user exists."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds'). Case-sensitive.")),
		),
		summarizeUser,
	)

	// Check transport mode from environment
	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio" // Default to stdio
	}

	switch transport {
	case "http":
		addr := os.Getenv("MCP_LISTEN_ADDR")
		if addr == "" {
			addr = ":8000"
		}
		startHTTPMode(s, addr)
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			panic(err)
		}
	default:
		panic(fmt.Sprintf("unknown transport: %s (must be 'stdio' or 'http')", transport))
	}
}

// startHTTPMode starts the MCP server in HTTP mode with a health check endpoint
func startHTTPMode(s *server.MCPServer, addr string) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// MCP endpoint
	httpServer := server.NewStreamableHTTPServer(s)
	mux.Handle("/mcp", httpServer)

	// Log startup
	fmt.Fprintf(os.Stderr, "Starting GitHub MCP server on http://localhost%s\n", addr)
	fmt.Fprintf(os.Stderr, "  MCP endpoint: http://localhost%s/mcp\n", addr)
	fmt.Fprintf(os.Stderr, "  Health check: http://localhost%s/healthz\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}
