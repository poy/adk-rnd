package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatalf("usage: %s [UPSTREAM_MCP_PATH] <UPSTREAM_MCP_ARGS...>", os.Args[0])
	}
	upstreamPath := os.Args[1]

	var args []string
	if len(os.Args) > 2 {
		args = append(args, os.Args[2:]...)
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

	// Initialize upstream and log capabilities.
	ctx := context.Background()
	initResp, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("upstream initialize failed: %v", err)
	}
	logJSON("upstream.initialize.response", initResp)

	// Fetch upstream tools to expose identical interface.
	listTools, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("upstream tools/list failed: %v", err)
	}
	logJSON("upstream.tools.list.response", listTools)

	// Build our proxy MCP server on stdio.
	s := server.NewMCPServer("passthrough-proxy", "1.0.0")

	// For each upstream tool, register a proxy handler that forwards the call.
	for _, t := range listTools.Tools {
		tool := t // capture
		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Log inbound request.
			logJSON("proxy.tools.call.request", struct {
				Name      string              `json:"name"`
				Arguments any                 `json:"arguments"`
				Raw       mcp.CallToolRequest `json:"raw"`
			}{
				Name:      req.Params.Name,
				Arguments: req.Params.Arguments,
				Raw:       req,
			})

			start := time.Now()
			res, err := mcpClient.CallTool(ctx, req)
			d := time.Since(start)

			if err != nil {
				logJSON("proxy.tools.call.error", struct {
					Name  string `json:"name"`
					Error string `json:"error"`
					MS    int64  `json:"elapsed_ms"`
				}{Name: req.Params.Name, Error: err.Error(), MS: d.Milliseconds()})
				// Return an MCP-formatted error result so the client gets something structured.
				return mcp.NewToolResultError(fmt.Sprintf("forward error: %v", err)), nil
			}

			// Log outbound response.
			logJSON("proxy.tools.call.response", struct {
				Name   string              `json:"name"`
				Result *mcp.CallToolResult `json:"result"`
				MS     int64               `json:"elapsed_ms"`
			}{Name: req.Params.Name, Result: res, MS: d.Milliseconds()})

			return res, nil
		})
		log.Printf("registered passthrough tool: %s", tool.Name)
	}

	log.Println("passthrough proxy MCP server running on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("ServeStdio error: %v", err)
	}
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

// logJSON prints a compact JSON record to stderr.
func logJSON(kind string, v any) {
	record := map[string]any{
		"ts":   time.Now().Format(time.RFC3339Nano),
		"type": kind,
		"data": v,
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(record); err != nil {
		log.Printf("json log encode error (%s): %v", kind, err)
	}
}
