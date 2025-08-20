package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	toolName := flag.String("toolName", "run_go", "The name of the tool")
	toolDescription := flag.String("toolDescription", "Run Go code from a main.go-style string", "The description of the tool")
	flag.Parse()

	srv := server.NewMCPServer("run-go", "v0.0.1")

	srv.AddTool(
		mcp.NewTool(*toolName,
			mcp.WithDescription(*toolDescription),
			mcp.WithString("source", mcp.Required(), mcp.Description("The Go source code (must contain a main function)")),
		),
		runGoHandler,
	)

	// Start the stdio server
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runGoHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source, err := req.RequireString("source")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tmpDir, err := os.MkdirTemp("", "go_run_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("failed to write main.go: %w", err)
	}

	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = tmpDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := struct {
		Success bool   `json:"success"`
		Output  string `json:"output"`
		Error   string `json:"error,omitempty"`
	}{
		Success: err == nil,
		Output:  strings.TrimSpace(stdout.String()),
	}

	if err != nil {
		result.Error = strings.TrimSpace(stderr.String())
	}

	jsonOutput, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Text: string(jsonOutput),
				Type: "text",
			},
		},
	}, nil
}
