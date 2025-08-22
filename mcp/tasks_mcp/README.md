# Tasks MCP Tool

A simple task management MCP server that allows you to create, update, mark complete, and list tasks.

## Tools Provided

- `add_task` - Add a new task that must get done
- `update_task_status` - Add a status update to an existing task  
- `mark_task_done` - Mark a task as complete with a final update
- `list_tasks` - List all tasks with their status updates

## Getting Started

Run the task management server:

```bash
go run .
```

This will start a stdio-based MCP server that provides task management capabilities.

## Usage Examples

### Adding a task
```json
{
  "name": "add_task",
  "arguments": {
    "description": "Implement user authentication system"
  }
}
```

### Updating task status
```json
{
  "name": "update_task_status", 
  "arguments": {
    "id": "task-12345",
    "statusUpdate": "Started working on OAuth integration"
  }
}
```

### Marking task complete
```json
{
  "name": "mark_task_done",
  "arguments": {
    "id": "task-12345", 
    "finalUpdate": "Authentication system completed and tested"
  }
}
```

### Listing all tasks
```json
{
  "name": "list_tasks",
  "arguments": {}
}
```

## Features

- In-memory task storage (tasks persist during server lifetime)
- Automatic task ID generation
- Timestamp tracking for task creation and status updates
- Status update history for each task