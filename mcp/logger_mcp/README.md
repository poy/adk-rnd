# Logger MCP Tool

An MCP proxy server that logs all tool calls, responses, and timing information while forwarding requests to an upstream MCP server.

## Purpose

This tool wraps any existing MCP server and provides detailed logging of all interactions, making it useful for debugging, monitoring, and understanding MCP tool usage patterns.

## Getting Started

Run the logger proxy with an upstream MCP server:

```bash
go run . path/to/upstream/mcp/server [upstream-args...]
```

For example, to log interactions with the tasks MCP server:

```bash
go run . ../tasks_mcp/tasks_mcp
```

## What Gets Logged

The logger captures and outputs JSON-structured logs for:

- **Initialization**: Upstream server capabilities and configuration
- **Tool Discovery**: List of available tools from upstream server  
- **Tool Calls**: Incoming requests with parameters and timing
- **Tool Responses**: Results from upstream server with execution time
- **Errors**: Any failures in forwarding or execution

## Log Format

All logs are JSON-structured with timestamps:

```json
{
  "ts": "2023-12-07T10:30:45.123456789Z",
  "type": "proxy.tools.call.request",
  "data": {
    "name": "add_task",
    "arguments": {"description": "Implement logging"},
    "raw": {...}
  }
}
```

## Usage Examples

### Logging task management operations
```bash
# Terminal 1: Start logged task server
go run . ../tasks_mcp/tasks_mcp

# Terminal 2: Use MCP client to interact
# All interactions will be logged to stderr
```

### Logging with Go code execution
```bash
go run . ../golang_run_mcp/golang_run_mcp
```

## Features

- **Transparent Proxying**: All upstream tools are exposed identically
- **Detailed Timing**: Millisecond-precision execution times
- **Error Tracking**: Comprehensive error logging and forwarding
- **Non-intrusive**: Does not modify tool behavior, only observes
- **Structured Logs**: JSON format for easy parsing and analysis
- **Stderr Mirroring**: Upstream server stderr is preserved and forwarded

## Use Cases

- Debugging MCP tool interactions
- Performance monitoring and optimization
- Understanding usage patterns
- Compliance and audit logging
- Development and testing workflows
- Troubleshooting integration issues

## Output

All logs go to stderr, allowing normal MCP communication on stdout while preserving detailed interaction logs for analysis.