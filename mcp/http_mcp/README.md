# HTTP MCP Tool

An MCP proxy server that exposes upstream MCP tools over HTTP instead of stdio, enabling web-based and HTTP client access to MCP tools.

## Purpose

This tool wraps any existing stdio-based MCP server and makes it accessible via HTTP with the MCP streaming protocol, allowing web applications and HTTP clients to interact with MCP tools.

## Getting Started

Run the HTTP proxy with an upstream MCP server:

```bash
go run . -addr=:8888 path/to/upstream/mcp/server [upstream-args...]
```

For example, to expose the tasks MCP server over HTTP:

```bash
go run . -addr=:8888 ../tasks_mcp/tasks_mcp
```

## Command Line Options

- `-addr` - Address to listen on (default: ":8888")
  - Examples: `:8888`, `127.0.0.1:9000`, `localhost:3000`

## Usage Examples

### Exposing task management over HTTP
```bash
# Start HTTP proxy for tasks server
go run . -addr=:8080 ../tasks_mcp/tasks_mcp

# Now accessible at http://localhost:8080
```

### Exposing Go code execution over HTTP  
```bash
# Start HTTP proxy for Go runner
go run . -addr=:9000 ../golang_run_mcp/golang_run_mcp

# Now accessible at http://localhost:9000
```

### Custom address binding
```bash
# Bind to specific interface
go run . -addr=127.0.0.1:8888 ../static_json_mcp/static_json_mcp -file=data.json
```

## Protocol

The server implements the MCP streaming protocol over HTTP, allowing:

- WebSocket-like persistent connections
- Real-time tool call streaming
- Heartbeat/keepalive functionality (1-second intervals)
- Graceful connection handling

## Features

- **HTTP/WebSocket Access**: Convert stdio MCP servers to HTTP endpoints
- **Transparent Proxying**: All upstream tools exposed identically  
- **Graceful Shutdown**: Clean shutdown on SIGINT/SIGTERM
- **Heartbeat Support**: Configurable connection keepalive
- **Error Forwarding**: Upstream errors properly formatted for HTTP clients
- **Concurrent Handling**: Multiple simultaneous HTTP connections supported

## Use Cases

- Web-based MCP tool interfaces
- Browser-based development tools
- HTTP API integration with existing MCP tools
- Cross-platform tool access
- Remote MCP server access
- Integration with web applications and services

## Security Considerations

- No built-in authentication - consider reverse proxy for production
- Binds to specified address/port - ensure firewall rules are appropriate
- Upstream tool security still applies