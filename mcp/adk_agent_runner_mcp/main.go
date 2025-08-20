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
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	toolName := flag.String("toolName", "adk_graph", "Tool name")
	toolDescription := flag.String("toolDescription", "Extracts agent and sub-agent relationships from an ADK Python script", "Tool description")
	flag.Parse()

	srv := server.NewMCPServer("adk-graph-tool", "v0.0.1")

	srv.AddTool(
		mcp.NewTool(*toolName,
			mcp.WithDescription(*toolDescription),
			mcp.WithString("source", mcp.Required(), mcp.Description("The ADK agent Python code (string, not path)")),
		),
		runHandler,
	)

	log.Printf("Serving tool %q...", *toolName)
	// Start the stdio server
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pyCode, err := req.RequireString("source")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tmpDir, err := os.MkdirTemp("", "adk_graph")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := tmpDir + "/extract.py"

	extractor := `
import ast
import sys
import json

class AgentGraphVisitor(ast.NodeVisitor):
    def __init__(self):
        self.root_agent = None
        self.subagents = {}

    def visit_Assign(self, node):
        if isinstance(node.value, ast.Call) and hasattr(node.value.func, "id"):
            class_name = node.value.func.id
            if class_name.endswith("Agent"):
                agent_name = node.targets[0].id
                self.subagents[agent_name] = class_name

        if isinstance(node.value, ast.Call) and hasattr(node.value.func, "id") and node.value.func.id == "StoryFlowAgent":
            if isinstance(node.targets[0], ast.Name):
                self.root_agent = node.targets[0].id
        self.generic_visit(node)

    def result(self):
        return {
            "root_agent": self.root_agent,
            "subagents": self.subagents
        }

code = sys.stdin.read()
tree = ast.parse(code)
visitor = AgentGraphVisitor()
visitor.visit(tree)
print(json.dumps(visitor.result(), indent=2))
`

	if err := os.WriteFile(scriptPath, []byte(extractor), 0644); err != nil {
		return nil, err
	}

	cmd := exec.Command("python3", scriptPath)
	cmd.Stdin = strings.NewReader(pyCode)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Text: "Error: " + stderr.String(), Type: "text"},
			},
		}, nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	result, _ := json.MarshalIndent(parsed, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Text: string(result), Type: "text"},
		},
	}, nil
}
