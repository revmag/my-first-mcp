package main

  import (
  	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var githubClient *GitHubClient

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

	out := strings.Join(repos, "\n")
	return mcp.NewToolResultText(out), nil
}

func main() {
	// Initialize GitHub client
	githubClient = NewGitHubClient("https://api.github.com", "")

	s := server.NewMCPServer("github-mcp", "1.0.0")

	tool := mcp.NewTool("list_repos",
		mcp.WithDescription("List public GitHub repositories for a user"),
		mcp.WithString("username", mcp.Required(), mcp.Description("GitHub username")),
	)

	s.AddTool(tool, listRepos)

	if err := server.ServeStdio(s); err != nil {
		panic(err)
	}
}