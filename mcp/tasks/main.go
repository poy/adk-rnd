package main

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Tasks",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	toolSet := tasksToolSet{
		tasks: make(map[string]*Task),
	}

	// Add tool
	s.AddTool(mcp.NewTool("add_task",
		mcp.WithDescription("Add a new task that must get done."),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description of the task"),
		),
	),
		toolSet.addTaskHandler)

	s.AddTool(mcp.NewTool("update_task_status",
		mcp.WithDescription("Add a new status update to a task"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("The ID of the task"),
		),
		mcp.WithString("statusUpdate",
			mcp.Required(),
			mcp.Description("The status update to add to the task"),
		),
	),
		toolSet.updateTaskStatusHandler)

	s.AddTool(mcp.NewTool("mark_task_done",
		mcp.WithDescription("Marks a task complete"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("The ID of the task"),
		),
		mcp.WithString("finalUpdate",
			mcp.Required(),
			mcp.Description("The final status update to add to the task"),
		),
	),
		toolSet.markTaskDoneHandler)

	s.AddTool(mcp.NewTool("list_tasks",
		mcp.WithDescription("Lists all the tasks"),
	),
		toolSet.listTasksHandler)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

type tasksToolSet struct {
	tasks map[string]*Task
}

type Task struct {
	ID           string
	Description  string
	StatusUpdate []StatusUpdate
	Created      time.Time
	Done         bool
}

type StatusUpdate struct {
	Description string
	Updated     time.Time
}

func (s *tasksToolSet) addTaskHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	desc, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	id := fmt.Sprintf("%d%d", time.Now().UnixNano(), rand.Uint64())

	s.tasks[id] = &Task{
		ID:          id,
		Created:     time.Now(),
		Description: desc,
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created task, %s", id)), nil
}

func (s *tasksToolSet) updateTaskStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	desc, err := request.RequireString("statusUpdate")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	task, ok := s.tasks[id]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown task with ID: %s", id)), nil
	}
	task.StatusUpdate = append(task.StatusUpdate, StatusUpdate{
		Description: desc,
		Updated:     time.Now(),
	})

	return mcp.NewToolResultText("Updated task status"), nil
}

func (s *tasksToolSet) markTaskDoneHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	desc, err := request.RequireString("finalUpdate")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	task, ok := s.tasks[id]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown task with ID: %s", id)), nil
	}
	task.StatusUpdate = append(task.StatusUpdate, StatusUpdate{
		Description: desc,
		Updated:     time.Now(),
	})
	task.Done = true

	return mcp.NewToolResultText("Updated task status"), nil
}

func (s *tasksToolSet) listTasksHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var results []*Task
	for _, task := range s.tasks {
		results = append(results, task)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Created.UnixNano() < results[j].Created.UnixNano()
	})

	var strResults []string
	for _, r := range results {
		complete := " "
		if r.Done {
			complete = "X"
		}
		strResults = append(strResults, fmt.Sprintf("- [%s] (id=%s) %s -> (%v)", complete, r.ID, r.Description, r.Created.Format("2006/01/02 15:04:05.000")))

		for _, u := range r.StatusUpdate {
			strResults = append(strResults, fmt.Sprintf("  - %s -> (%v)", u.Description, u.Updated.Format("2006/01/02 15:04:05.000")))
		}
	}

	return mcp.NewToolResultText(strings.Join(strResults, "\n")), nil
}
