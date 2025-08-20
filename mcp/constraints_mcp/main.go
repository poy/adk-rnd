package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/cel-go/cel"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 3 {
		log.Fatalf("usage: %s [CONSTRAINTS_PATH] [UPSTREAM_MCP_PATH] <UPSTREAM_MCP_ARGS...>", os.Args[0])
	}
	constraintsPath := os.Args[1]
	upstreamPath := os.Args[2]

	var args []string
	if len(os.Args) > 2 {
		args = append(args, os.Args[3:]...)
	}

	constraints, err := loadConstraints(constraintsPath)
	if err != nil {
		log.Fatalf("failed to load constraints: %v", err)
	}

	// Start upstream MCP over stdio.
	mcpClient, err := client.NewStdioMCPClient(upstreamPath, nil, args...)
	if err != nil {
		log.Fatalf("failed to start upstream: %v", err)
	}
	defer func() {
		_ = mcpClient.Close()
	}()
	// Mirror upstream stderr verbatim to our stderr.
	if r, ok := client.GetStderr(mcpClient); ok && r != nil {
		go mirrorStderr("upstream", r)
	}

	ctx := context.Background()
	if _, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		log.Fatalf("upstream initialize failed: %v", err)
	}

	// Fetch upstream tools to expose identical interface.
	listTools, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("upstream tools/list failed: %v", err)
	}

	// Build our proxy MCP server on stdio.
	s := server.NewMCPServer("passthrough-proxy", "1.0.0")

	// For each upstream tool, register a proxy handler that forwards the call if
	// it passes the given constraint.
	for _, t := range listTools.Tools {
		tool := t // capture
		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if ok, err := evalConstraint(constraints[t.Name], req.GetArguments()); err != nil {
				return mcp.NewToolResultErrorf("constraint failed to evaluate: %v", err), nil
			} else if !ok {
				return mcp.NewToolResultError("constraint returned false"), nil
			}

			res, err := mcpClient.CallTool(ctx, req)

			if err != nil {
				// Return an MCP-formatted error result so the client gets something structured.
				return mcp.NewToolResultError(fmt.Sprintf("forward error: %v", err)), nil
			}

			return res, nil
		})
		log.Printf("registered passthrough tool: %s", tool.Name)
	}
}

func loadConstraints(p string) (map[string]string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	var c map[string]string
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal constraints: %w", err)
	}
	return c, nil
}

func evalConstraint(constraintExpr string, args map[string]any) (bool, error) {
	if constraintExpr == "" {
		return true, nil
	}

	env, err := cel.NewEnv(
		cel.Variable("args", cel.DynType),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL env: %w", err)
	}

	ast, issues := env.Compile(constraintExpr)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("failed to compile CEL: %w", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL program: %w", err)
	}

	out, _, err := prg.Eval(map[string]any{
		"args": args,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression: %w", err)
	}

	// Expecting the output to be a boolean (true/false)
	boolVal, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL constraint did not return a boolean: got %T", out.Value())
	}

	return boolVal, nil
}

// mirrorStderr copies upstream stderr to our stderr, line-buffered, with a prefix.
func mirrorStderr(prefix string, r io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// We do not attempt to parse; just forward with a tag.
			os.Stderr.Write([]byte(fmt.Sprintf("[%s-stderr] ", prefix)))
			os.Stderr.Write(chunk)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("stderr mirror error: %v", err)
			}
			return
		}
	}
}
