package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/cel-go/cel"
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

func main() {
	log.SetFlags(0)
	var mcpPath string
	var constraintsPath string
	switch len(os.Args) {
	case 2:
		mcpPath = os.Args[1]
	case 3:
		mcpPath = os.Args[1]
		constraintsPath = os.Args[2]
	default:
		log.Fatalf("Usage: %s [MCP_PATH] <CONSTRAINTS_PATH>", os.Args[0])
	}

	var constraints map[string]string
	if constraintsPath != "" {
		var err error
		constraints, err = loadConstraints(constraintsPath)
		if err != nil {
			log.Fatalf("failed to load constraints: %v", err)
		}
	}

	ctx := context.Background()

	var err error
	mcpClient, err = client.NewStdioMCPClient(mcpPath, nil)
	if err != nil {
		log.Fatalf("Failed to start real-tool: %v", err)
	}
	defer mcpClient.Close()

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

	proxy.AddTool(mcp.NewTool("check_long_running_task",
		mcp.WithDescription("Checks to see if a long running task is done or still pending. If it's done, it will output the result."),
		mcp.WithString("id", mcp.Required(), mcp.Description("The ID of the long running task")),
	), checkLongRunningTaskHandler)

	for _, t := range listRes.Tools {
		proxy.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return consentProxyHandler(ctx, req, t.Name, constraints[t.Name])
		})
		log.Printf("Registered proxy tool: %s", t.Name)
	}

	go startHTTPServer()

	log.Println("Consent proxy MCP server running on stdio...")
	if err := server.ServeStdio(proxy); err != nil {
		log.Fatalf("ServeStdio error: %v", err)
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

func consentProxyHandler(ctx context.Context, req mcp.CallToolRequest, toolName, constraintExpr string) (*mcp.CallToolResult, error) {
	log.Printf("Proxying for %s", toolName)

	if ok, err := evalConstraint(constraintExpr, req.GetArguments()); err != nil {
		return mcp.NewToolResultErrorf("constraint failed to evaluate: %v", err), nil
	} else if !ok {
		return mcp.NewToolResultError("constraint returned false"), nil
	}

	return startLongRunningTask(func() *mcp.CallToolResult {

		callQueueLock.Lock()
		id := nextCallID
		nextCallID++
		pc := &pendingCall{ID: id, Request: req, ResponseC: make(chan *mcp.CallToolResult)}
		callQueue[id] = pc
		callQueueLock.Unlock()

		select {
		case result := <-pc.ResponseC:
			return result
		case <-ctx.Done():
			return mcp.NewToolResultError("Cancelled while waiting for approval")
		}
	}), nil
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
