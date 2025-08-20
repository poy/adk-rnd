package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/poy/adk-rnd/mcp/sqlite_mcp/pkg/mcpserver"
)

func TestCreateAndQuerySQLite(t *testing.T) {
	server := mcpserver.New(t.TempDir())
	tx := transport.NewInProcessTransport(server)
	mcpClient := client.NewClient(tx)
	if _, err := mcpClient.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name: "create_db",
		},
	}
	createRes, err := mcpClient.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("create_db failed: %v", err)
	}

	if len(createRes.Content) == 0 {
		t.Fatal("expected content in response")
	}

	var created struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal([]byte(createRes.Content[0].(mcp.TextContent).Text), &created); err != nil {
		t.Fatalf("failed to unmarshal session ID: %v", err)
	}
}

func TestRunSQLWithSession(t *testing.T) {
	server := mcpserver.New(t.TempDir())
	tx := transport.NewInProcessTransport(server)
	mcpClient := client.NewClient(tx)
	if _, err := mcpClient.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}

	// Create the DB
	createReq := mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params:  mcp.CallToolParams{Name: "create_db"},
	}
	createRes, err := mcpClient.CallTool(context.Background(), createReq)
	if err != nil {
		t.Fatalf("create_db failed: %v", err)
	}
	var created struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal([]byte(createRes.Content[0].(mcp.TextContent).Text), &created); err != nil {
		t.Fatalf("failed to unmarshal session ID: %v", err)
	}
	if created.Session == "" {
		t.Fatal("expected session to be returned")
	}

	// Run a create table + insert
	sqlStatements := []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);",
		"INSERT INTO users (name) VALUES ('Alice');",
		"INSERT INTO users (name) VALUES ('Bob');",
	}
	for _, stmt := range sqlStatements {
		runReq := mcp.CallToolRequest{
			Request: mcp.Request{Method: "tools/call"},
			Params: mcp.CallToolParams{
				Name: "run_sql",
				Arguments: map[string]string{
					"session": created.Session,
					"sql":     stmt,
				},
			},
		}
		_, err := mcpClient.CallTool(context.Background(), runReq)
		if err != nil {
			t.Fatalf("run_sql failed: %v", err)
		}
	}

	// Select data
	selectReq := mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "run_sql",
			Arguments: map[string]string{
				"session": created.Session,
				"sql":     "SELECT name FROM users ORDER BY id;",
			},
		},
	}
	selectRes, err := mcpClient.CallTool(context.Background(), selectReq)
	if err != nil {
		t.Fatalf("run_sql select failed: %v", err)
	}

	var out struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(selectRes.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("failed to unmarshal result JSON: %v", err)
	}

	if len(out.Results) != 2 || out.Results[0]["name"] != "Alice" || out.Results[1]["name"] != "Bob" {
		t.Fatalf("unexpected query results: %+v", out.Results)
	}
}
