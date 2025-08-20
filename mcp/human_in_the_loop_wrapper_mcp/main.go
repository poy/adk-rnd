package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type pendingCall struct {
	ID        int
	Request   mcp.CallToolRequest
	ResponseC chan *mcp.CallToolResult
}

var (
	callQueue     = make(map[int]*pendingCall)
	callQueueLock sync.Mutex
	nextCallID    = 1
	mcpClient     *client.Client
)

type MethodConfig struct {
	MethodName string `json:"methodName"`
	Enabled    bool   `json:"enabled"`
}

func main() {
	println("HITL 0")
	log.SetFlags(0)

	if len(os.Args) < 3 {
		log.Fatalf("usage: %s [CONFIG_PATH] [UPSTREAM_MCP_PATH] <UPSTREAM_MCP_ARGS...>", os.Args[0])
	}

	configs, err := loadConfig(os.Args[1])
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	upstreamPath := os.Args[2]
	ctx := context.Background()

	var args []string
	if len(os.Args) > 2 {
		args = append(args, os.Args[3:]...)
	}

	println("HITL 1")
	// Start upstream MCP over stdio.
	mcpClient, err = client.NewStdioMCPClient(upstreamPath, nil, args...)
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

	errReader, ok := client.GetStderr(mcpClient)
	if ok {
		go func() {
			var data []byte
			if n, err := errReader.Read(data); err != nil {
				log.Printf("Failed to read from error reader: %v", err)
			} else if n > 0 {
				log.Printf("From error reader: %s", data[:n])
			}
		}()
	}

	initResp, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}
	json.NewEncoder(os.Stderr).Encode(initResp)

	listRes, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("tools/list failed: %v", err)
	}
	json.NewEncoder(os.Stderr).Encode(listRes)

	proxy := server.NewMCPServer("ConsentProxy", "1.0.0", server.WithToolCapabilities(false))

	for _, t := range listRes.Tools {
		proxy.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return consentProxyHandler(ctx, req, t.Name, configs)
		})
		log.Printf("Registered proxy tool: %s", t.Name)
	}

	go startHTTPServer()

	log.Println("Consent proxy MCP server running on stdio...")
	if err := server.ServeStdio(proxy); err != nil {
		log.Fatalf("ServeStdio error: %v", err)
	}
}

func loadConfig(p string) (map[string]MethodConfig, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	var cs []MethodConfig
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	m := map[string]MethodConfig{}
	for _, c := range cs {
		m[c.MethodName] = c
	}

	return m, nil
}

func consentProxyHandler(ctx context.Context, req mcp.CallToolRequest, toolName string, configs map[string]MethodConfig) (*mcp.CallToolResult, error) {
	log.Printf("Proxying for %s", toolName)
	if !configs[toolName].Enabled {
		return mcpClient.CallTool(ctx, req)
	}

	callQueueLock.Lock()
	id := nextCallID
	nextCallID++
	pc := &pendingCall{ID: id, Request: req, ResponseC: make(chan *mcp.CallToolResult)}
	callQueue[id] = pc
	callQueueLock.Unlock()

	select {
	case result := <-pc.ResponseC:
		return result, nil
	case <-ctx.Done():
		return mcp.NewToolResultError("Cancelled while waiting for approval"), nil
	}
}

func startHTTPServer() {
	http.HandleFunc("/", listPendingCalls)
	http.HandleFunc("/approve", handleApproval(true))
	http.HandleFunc("/reject", handleApproval(false))

	log.Println("HTTP approval UI at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func listPendingCalls(w http.ResponseWriter, r *http.Request) {
	callQueueLock.Lock()
	defer callQueueLock.Unlock()
	type row struct {
		ID   int
		Tool string
		Args string
	}
	var rows []row
	for _, pc := range callQueue {
		args, _ := json.MarshalIndent(pc.Request.Params.Arguments, "", "  ")
		rows = append(rows, row{ID: pc.ID, Tool: pc.Request.Params.Name, Args: string(args)})
	}
	tmpl := `
<html>
<head><title>Pending MCP Tool Calls</title><meta http-equiv="refresh" content="5">
<style>
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #ccc; padding: 8px; }
</style>
</head>
<body>
  <h2>Pending Tool Calls</h2>
  <table>
    <tr><th>ID</th><th>Tool</th><th>Arguments</th><th>Action</th></tr>
    {{range .}}
    <tr>
      <td>{{.ID}}</td>
      <td>{{.Tool}}</td>
      <td><pre>{{.Args}}</pre></td>
      <td>
        <a href="/approve?id={{.ID}}">✅ Approve</a> |
        <a href="/reject?id={{.ID}}">❌ Reject</a>
      </td>
    </tr>
    {{else}}
    <tr><td colspan="4">No pending calls</td></tr>
    {{end}}
  </table>
</body>
</html>`
	t := template.Must(template.New("page").Parse(tmpl))
	t.Execute(w, rows)
}

func handleApproval(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		callQueueLock.Lock()
		pc := callQueue[id]
		delete(callQueue, id)
		callQueueLock.Unlock()
		if pc == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if approve {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := mcpClient.CallTool(ctx, pc.Request)
			if err != nil {
				pc.ResponseC <- mcp.NewToolResultError(fmt.Sprintf("Forward error: %v", err))
			} else {
				pc.ResponseC <- res
			}
		} else {
			pc.ResponseC <- mcp.NewToolResultError("User rejected the request")
		}
		http.Redirect(w, r, "/", http.StatusFound)
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
