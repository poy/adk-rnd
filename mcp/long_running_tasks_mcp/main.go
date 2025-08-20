package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MethodConfig struct {
	MethodName string `json:"methodName"`
	Enabled    bool   `json:"enabled"`
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s [CONFIG_PATH] [UPSTREAM_MCP_PATH] <UPSTREAM_MCP_ARGS...>", os.Args[0])
	}

	upstreamPath := os.Args[2]

	configs, err := loadConfig(os.Args[1])
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	lroMethods := map[string]struct{}{}
	for _, c := range configs {
		if !c.Enabled {
			continue
		}
		log.Printf("putting %s behind a LRO", c.MethodName)
		lroMethods[c.MethodName] = struct{}{}
	}

	var args []string
	if len(os.Args) > 2 {
		args = append(args, os.Args[3:]...)
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

	s.AddTool(mcp.NewTool("check_long_running_task",
		mcp.WithDescription("Checks to see if a long running task is done or still pending. If it's done, it will output the result."),
		mcp.WithString("id", mcp.Required(), mcp.Description("The ID of the long running task")),
	), checkLongRunningTaskHandler)

	// For each upstream tool, register a proxy handler that forwards the call.
	for _, t := range listTools.Tools {
		tool := t
		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if _, ok := lroMethods[t.Name]; !ok {
				log.Printf("Not putting %s behind a LRO", t.Name)
				res, err := mcpClient.CallTool(ctx, req)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("forward error: %v", err)), nil
				}
				return res, nil
			}

			log.Printf("Putting %s behind a LRO", t.Name)

			return startLongRunningTask(func() *mcp.CallToolResult {
				res, err := mcpClient.CallTool(ctx, req)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("forward error: %v", err))
				}
				return res
			}), nil
		})
		log.Printf("registered passthrough tool: %s", tool.Name)
	}

	log.Println("long running tasks: passthrough proxy MCP server running on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("ServeStdio error: %v", err)
	}
}

type LongRunningTaskStatus int

func (s LongRunningTaskStatus) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Done:
		return "Done"
	default:
		return strconv.FormatInt(int64(s), 10)
	}
}

const (
	Pending LongRunningTaskStatus = iota
	Done
)

var longRunningTasks sync.Map

func checkLongRunningTaskHandler(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireString("id")
	if err != nil {
		return mcp.NewToolResultErrorFromErr("missing required argument", err), nil
	}
	log.Printf("Check long running tasks at ID %s", id)

	val, ok := longRunningTasks.Load(id)
	if !ok {
		return mcp.NewToolResultErrorf("unknown task ID %s", id), nil
	}
	t := val.(*LongRunningTask)
	switch status := t.Status(); status {
	case Pending:
		// We force a cooldown here.
		log.Printf("Task %s is still pending, sleeping 3 seconds...", id)
		time.Sleep(3 * time.Second)
		return mcp.NewToolResultText(fmt.Sprintf("Task %s is pending", id)), nil
	case Done:
		result := t.Result()

		log.Printf("Task %s is done", id)
		return result, nil
	default:
		panic(fmt.Sprintf("unknown task status: %v", status))
	}
}

func startLongRunningTask(f func() *mcp.CallToolResult) *mcp.CallToolResult {
	t := Run(f)
	longRunningTasks.Store(t.ID, t)
	return mcp.NewToolResultStructured(struct {
		LongRunningTaskID string `json:"long_running_task_id"`
	}{
		LongRunningTaskID: t.ID,
	}, fmt.Sprintf("Started long running task with ID: %s", t.ID))
}

type LongRunningTask struct {
	mu     sync.Mutex
	ID     string
	status LongRunningTaskStatus
	result *mcp.CallToolResult
}

func (t *LongRunningTask) Status() LongRunningTaskStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func (t *LongRunningTask) Result() *mcp.CallToolResult {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result
}

var nextID uint64

func Run(f func() *mcp.CallToolResult) *LongRunningTask {
	t := &LongRunningTask{
		ID:     fmt.Sprintf("%d", atomic.AddUint64(&nextID, 1)),
		status: Pending,
	}
	go func() {
		out := f()
		t.mu.Lock()
		defer t.mu.Unlock()

		t.status = Done
		t.result = out
	}()
	return t
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
