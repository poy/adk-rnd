# adk-rnd

A playground for rapid prototyping, research, and deep dives with the Agent Deployment Kit (ADK).

## MCP Tools

This repository contains a collection of Model Context Protocol (MCP) tools for various development and operations tasks. Each tool can be used standalone or chained together for complex workflows.

### Available Tools

| Tool | Purpose | Key Features |
|------|---------|--------------|
| [tasks_mcp](mcp/tasks_mcp) | Task management | Create, update, complete, and list tasks |
| [golang_run_mcp](mcp/golang_run_mcp) | Go code execution | Run Go code from string input in isolated environment |
| [adk_agent_runner_mcp](mcp/adk_agent_runner_mcp) | ADK analysis | Extract agent relationships from ADK Python scripts |
| [static_json_mcp](mcp/static_json_mcp) | JSON data serving | Serve paginated JSON data with auto-inferred schema |
| [sqlite_mcp](mcp/sqlite_mcp) | Database operations | SQLite database with session management |
| [logger_mcp](mcp/logger_mcp) | Logging proxy | Log all MCP interactions for debugging/monitoring |
| [http_mcp](mcp/http_mcp) | HTTP proxy | Expose MCP tools over HTTP instead of stdio |
| [constraints_mcp](mcp/constraints_mcp) | Validation proxy | Apply CEL constraints to tool calls |
| [long_running_tasks_mcp](mcp/long_running_tasks_mcp) | Async operations | Convert tools to long-running tasks with polling |
| [human_in_the_loop_wrapper_mcp](mcp/human_in_the_loop_wrapper_mcp) | Human approval | Require manual approval for tool calls via web UI |

### Core Tools vs. Wrapper Tools

**Core Tools** provide direct functionality:
- `tasks_mcp` - Task management
- `golang_run_mcp` - Go code execution  
- `adk_agent_runner_mcp` - ADK analysis
- `static_json_mcp` - JSON data serving
- `sqlite_mcp` - Database operations

**Wrapper Tools** enhance other tools with additional capabilities:
- `logger_mcp` - Adds logging to any MCP tool
- `http_mcp` - Exposes any MCP tool over HTTP
- `constraints_mcp` - Adds validation to any MCP tool
- `long_running_tasks_mcp` - Adds async execution to any MCP tool
- `human_in_the_loop_wrapper_mcp` - Adds manual approval to any MCP tool

## Tool Chaining Examples

### Example 1: Logged Task Management over HTTP

Create a task management system that's accessible over HTTP with full logging:

```bash
# Terminal 1: Start the chain
cd mcp/tasks_mcp && go build && cd ..
cd logger_mcp && go run . ../tasks_mcp/tasks_mcp > task_logs.json 2>&1 &
cd ../http_mcp && go run . -addr=:8080 ../logger_mcp/logger_mcp ../tasks_mcp/tasks_mcp
```

This creates: HTTP ← Logger ← Tasks
- Tasks are accessible at http://localhost:8080
- All interactions are logged to `task_logs.json`

### Example 2: Constrained Go Code Execution with Human Approval

Create a safe Go code execution environment with constraints and human approval:

```bash
# Create constraints file
echo '{
  "run_go": "!contains(args.source, \"os.Exit\") && !contains(args.source, \"exec.Command\")"
}' > go_constraints.json

# Create HITL config  
echo '{
  "run_go": {"methodName": "run_go", "enabled": true}
}' > hitl_config.json

# Start the chain
cd mcp/golang_run_mcp && go build && cd ..
cd constraints_mcp && go run . ../go_constraints.json ../golang_run_mcp/golang_run_mcp &
cd ../human_in_the_loop_wrapper_mcp && go run . ../hitl_config.json ../constraints_mcp/constraints_mcp ../go_constraints.json ../golang_run_mcp/golang_run_mcp
```

This creates: HITL ← Constraints ← Go Runner
- Go code is validated for dangerous patterns
- Approved code requires manual confirmation at http://localhost:8080
- Only safe, approved code gets executed

### Example 3: Long-Running Database Operations with Logging

Set up a database system that can handle long-running queries with full logging:

```bash
# Create LRO config for complex queries
echo '{
  "run_sql": {"methodName": "run_sql", "enabled": true}
}' > db_lro_config.json

# Start the chain
cd mcp/sqlite_mcp && go build ./cmd/sqlite_mcp && cd ..
cd long_running_tasks_mcp && go run . ../db_lro_config.json ../sqlite_mcp/sqlite_mcp &
cd ../logger_mcp && go run . ../long_running_tasks_mcp/long_running_tasks_mcp ../db_lro_config.json ../sqlite_mcp/sqlite_mcp
```

This creates: Logger ← LRO ← SQLite
- Database operations can run asynchronously
- Long queries don't block the client
- All database interactions are logged

### Example 4: Full-Stack Development Environment

Combine multiple tools for a complete development workflow:

```bash
# Start core services
cd mcp/tasks_mcp && go build && cd ..
cd mcp/golang_run_mcp && go build && cd ..
cd mcp/sqlite_mcp && go build ./cmd/sqlite_mcp && cd ..

# Create task management with logging
cd logger_mcp && go run . ../tasks_mcp/tasks_mcp > tasks.log 2>&1 &

# Create HTTP-accessible Go execution with constraints  
echo '{"run_go": "size(args.source) < 10000"}' > go_size_limit.json
cd constraints_mcp && go run . ../go_size_limit.json ../golang_run_mcp/golang_run_mcp &
cd ../http_mcp && go run . -addr=:8081 ../constraints_mcp/constraints_mcp ../go_size_limit.json ../golang_run_mcp/golang_run_mcp &

# Create database with session management over HTTP
cd http_mcp && go run . -addr=:8082 ../sqlite_mcp/sqlite_mcp
```

This provides:
- Task management with logging (stdio)
- Size-limited Go execution over HTTP (port 8081)  
- Database operations over HTTP (port 8082)

## Getting Started

1. **Choose your core functionality** from the available tools
2. **Add wrappers** for additional capabilities (logging, HTTP, constraints, etc.)
3. **Chain tools** by having wrapper tools call upstream tools
4. **Configure** each tool according to its README

Each tool directory contains a detailed README with usage examples and configuration options.
