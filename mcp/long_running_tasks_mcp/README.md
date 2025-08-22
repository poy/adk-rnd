# Long Running Tasks MCP Tool

An MCP proxy server that wraps upstream MCP tools to provide long-running task support with asynchronous execution and polling.

## Purpose

This tool wraps any existing MCP server and converts specified tools into long-running operations that can be started, then polled for completion. This is useful for time-consuming operations that would otherwise timeout or block.

## Getting Started

Create a configuration file specifying which tools should be long-running:

```json
{
  "run_go": {"methodName": "run_go", "enabled": true},
  "complex_analysis": {"methodName": "complex_analysis", "enabled": true}
}
```

Run the long-running tasks proxy:

```bash
go run . config.json path/to/upstream/mcp/server [upstream-args...]
```

## Configuration Format

The configuration file maps tool names to their long-running settings:

```json
{
  "tool_name": {
    "methodName": "tool_name",
    "enabled": true
  }
}
```

## Tools Provided

- **All upstream tools**: Either as direct pass-through or long-running versions
- **check_long_running_task**: Poll for completion of long-running tasks

## Usage Examples

### Starting a long-running task
```json
{
  "name": "run_go",
  "arguments": {
    "source": "package main\n\nimport \"time\"\n\nfunc main() {\n    time.Sleep(30 * time.Second)\n    println(\"Done!\")\n}"
  }
}
```

Response for long-running tools:
```json
{
  "task_id": "task-12345",
  "status": "running",
  "message": "Task started successfully"
}
```

### Checking task status
```json
{
  "name": "check_long_running_task",
  "arguments": {
    "id": "task-12345"
  }
}
```

Response when still running:
```json
{
  "status": "running",
  "task_id": "task-12345"
}
```

Response when complete:
```json
{
  "status": "completed",
  "task_id": "task-12345",
  "result": {
    "success": true,
    "output": "Done!"
  }
}
```

## Task Status States

- **running**: Task is still executing
- **completed**: Task finished successfully with results
- **failed**: Task encountered an error

## Features

- **Selective LRO**: Only configured tools become long-running
- **Automatic Task IDs**: Unique identifiers for each long-running task
- **Async Execution**: Tasks run in background goroutines
- **Result Caching**: Completed results are stored for retrieval
- **Transparent Pass-through**: Non-LRO tools work normally
- **Status Polling**: Check task progress at any time

## Use Cases

- **Long-running Computations**: Complex analysis or processing
- **File Operations**: Large file uploads, downloads, or processing
- **External API Calls**: Operations that may take significant time
- **Batch Processing**: Operations on large datasets
- **Background Jobs**: Tasks that don't need immediate results
- **Timeout Prevention**: Avoid timeout issues with slow operations

## Configuration Example

For a Go code execution server with long-running support:

```json
{
  "run_go": {
    "methodName": "run_go", 
    "enabled": true
  }
}
```

Then run:
```bash
go run . lro_config.json ../golang_run_mcp/golang_run_mcp
```

This allows Go code that takes a long time to execute without blocking the client.