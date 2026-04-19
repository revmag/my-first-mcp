package main

  import (
  	"context"
  	"encoding/json"
  	"fmt"
  	"io"
  	"net/http"

  	"github.com/mark3labs/mcp-go/mcp"
  	"github.com/mark3labs/mcp-go/server"
  )

  func listRepos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
  	username, err := req.RequireString("username")
  	if err != nil {
  		return mcp.NewToolResultError(err.Error()), nil
  	}

  	url := fmt.Sprintf("https://api.github.com/users/%s/repos", username)
  	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
  	if err != nil {
  		return mcp.NewToolResultError(err.Error()), nil
  	}
  	httpReq.Header.Set("Accept", "application/vnd.github+json")

  	resp, err := http.DefaultClient.Do(httpReq)
  	if err != nil {
  		return mcp.NewToolResultError(err.Error()), nil
  	}
  	defer resp.Body.Close()

  	if resp.StatusCode != http.StatusOK {
  		body, _ := io.ReadAll(resp.Body)
  		return mcp.NewToolResultError(fmt.Sprintf("github api error: %s %s", resp.Status, string(body))), nil
  	}

  	var repos []struct {
  		Name string `json:"name"`
  	}
  	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
  		return mcp.NewToolResultError(err.Error()), nil
  	}

  	out := ""
  	for i, r := range repos {
  		if i >= 10 {
  			break
  		}
  		out += r.Name + "\n"
  	}
  	if out == "" {
  		out = "no repos found"
  	}

  	return mcp.NewToolResultText(out), nil
  }

  func main() {
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