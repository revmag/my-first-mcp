package main

import (
	"context"
	"fmt"
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
		return mcp.NewToolResultError(err.Error()), nil
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	detail, err := githubClient.GetRepo(ctx, owner, repo)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	language := optString(req, "language")
	sortBy := optString(req, "sort")
	limit := optInt(req, "limit", 10)

	results, err := githubClient.SearchRepos(ctx, query, language, sortBy, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	p, err := githubClient.GetUserProfile(ctx, username)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	s, err := githubClient.SummarizeUser(ctx, username)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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

	s.AddTool(
		mcp.NewTool("list_repos",
			mcp.WithDescription("List up to 10 public repository names for a GitHub user. Use this for a quick overview of what repos a user has. Returns only names — use get_repo for full details on a specific repo."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds', 'octocat')")),
		),
		listRepos,
	)

	s.AddTool(
		mcp.NewTool("get_repo",
			mcp.WithDescription("Fetch full metadata for a single GitHub repository. Use when you know the exact owner and repo name and need details like star count, forks, open issues, primary language, description, default branch, or last updated time. Not for discovery — use search_repos if the exact name is unknown."),
			mcp.WithString("owner", mcp.Required(), mcp.Description("GitHub login of the repo owner (e.g. 'torvalds')")),
			mcp.WithString("repo", mcp.Required(), mcp.Description("Exact repository name (e.g. 'linux')")),
		),
		getRepo,
	)

	s.AddTool(
		mcp.NewTool("search_repos",
			mcp.WithDescription("Search GitHub repositories by keyword. Use when the user wants to discover repos matching a topic, technology, or project name rather than looking up a known repo. Optionally filter by language (e.g. 'go', 'python', 'rust') and sort by 'stars', 'forks', or 'updated'. Example: query='kubernetes operator', language='go', sort='stars', limit=5."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search keywords (e.g. 'http server', 'machine learning framework')")),
			mcp.WithString("language", mcp.Description("Filter by primary programming language (e.g. 'go', 'python', 'typescript'). Omit to search all languages.")),
			mcp.WithString("sort", mcp.Description("Sort order: 'stars' (default), 'forks', or 'updated'")),
			mcp.WithNumber("limit", mcp.Description("Number of results to return (1-20, default 10)")),
		),
		searchRepos,
	)

	s.AddTool(
		mcp.NewTool("get_user_profile",
			mcp.WithDescription("Fetch a GitHub user's public profile: display name, bio, company, location, follower count, following count, and total public repo count. Use when the user asks who someone is or wants identity/social info. NOT for listing repos — use list_repos for that."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds')")),
		),
		getUserProfile,
	)

	s.AddTool(
		mcp.NewTool("summarize_user",
			mcp.WithDescription("Aggregate statistics across all public repos for a GitHub user: total repo count, cumulative star count, and a ranked breakdown of programming languages used. Use when the user asks for a high-level overview of a developer's GitHub presence or tech stack. More comprehensive than list_repos — fetches up to 100 repos to compute the summary."),
			mcp.WithString("username", mcp.Required(), mcp.Description("GitHub login (e.g. 'torvalds')")),
		),
		summarizeUser,
	)

	if err := server.ServeStdio(s); err != nil {
		panic(err)
	}
}
