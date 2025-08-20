package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/poy/adk-rnd/mcp/sqlite_mcp/pkg/sessionmanager"
)

func New(dataDir string) *server.MCPServer {
	mgr := sessionmanager.NewSessionManager(dataDir, 15*time.Minute)

	s := &handlers{
		manager: mgr,
	}

	server := server.NewMCPServer("SQLite", "v0.0.1")
	server.AddTool(mcp.NewTool("create_db",
		mcp.WithDescription("Create a new SQLite database session. This will provide a session that will be used with other method calls"),
	), s.createDBHandler)
	server.AddTool(mcp.NewTool("run_sql",
		mcp.WithDescription("Execute a SQL statement against a session database"),
		mcp.WithString("session",
			mcp.Required(),
			mcp.Description("Session ID returned after you create a database with create_db"),
		),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL statement to run. Must only be a single SQL statement."),
		),
	), s.runSQLHandler)

	return server
}

type handlers struct {
	manager *sessionmanager.SessionManager
}

func (s *handlers) createDBHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := s.manager.CreateDatabase()
	if err != nil {
		log.Printf("failed to create db: %v", err)
		return nil, err
	}

	resp := map[string]string{
		"session": sessionID,
	}
	contentBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Text: string(contentBytes),
				Type: "text",
			},
		},
	}, nil
}

func (s *handlers) runSQLHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]any)
	session := args["session"].(string)
	sqlStmt := args["sql"].(string)

	if session == "" || sqlStmt == "" {
		return nil, fmt.Errorf("missing required parameters 'session' or 'sql'")
	}

	db, err := s.manager.GetDB(session)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(sqlStmt)
	if err != nil {
		// If it's not a query, try Exec (e.g. INSERT, CREATE, etc)
		if _, execErr := db.Exec(sqlStmt); execErr != nil {
			return nil, fmt.Errorf("sql error: %w", execErr)
		}
		// Return an empty result to indicate success
		resp := map[string]any{
			"result": "ok",
		}
		jsonBytes, _ := json.Marshal(resp)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(jsonBytes),
				},
			},
		}, nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any

	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any)
		for i, colName := range cols {
			switch v := raw[i].(type) {
			case nil:
				row[colName] = nil
			case []byte:
				row[colName] = string(v)
			default:
				row[colName] = v
			}
		}
		results = append(results, row)
	}

	resp := map[string]any{
		"results": results,
	}
	jsonBytes, _ := json.Marshal(resp)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(jsonBytes),
			},
		},
	}, nil
}
