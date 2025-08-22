# Constraints MCP Tool

An MCP proxy server that applies CEL (Common Expression Language) constraints to tool calls before forwarding them to an upstream MCP server.

## Purpose

This tool wraps any existing MCP server and adds constraint validation using CEL expressions, allowing you to control when and how tools can be called based on their arguments.

## Getting Started

Create a constraints configuration file (JSON format):

```json
{
  "add_task": "size(args.description) > 5",
  "update_task_status": "args.id != '' && size(args.statusUpdate) > 0",
  "run_go": "!has(args.source) || !contains(args.source, 'os.Exit')"
}
```

Run the constraints proxy:

```bash
go run . constraints.json path/to/upstream/mcp/server [upstream-args...]
```

## Configuration Format

The constraints file is a JSON object mapping tool names to CEL expressions:

```json
{
  "tool_name": "CEL_expression_here",
  "another_tool": "args.value > 0 && args.value < 100"
}
```

### CEL Expression Context

- `args` - Map of tool call arguments
- Standard CEL functions and operators
- Return value must be boolean (true = allow, false = reject)

## Usage Examples

### Basic validation constraints
```json
{
  "add_task": "size(args.description) >= 10",
  "delete_user": "args.id != 'admin'", 
  "run_code": "!contains(args.source, 'rm -rf')"
}
```

### Complex conditional constraints
```json
{
  "update_balance": "args.amount > 0 && args.amount <= 10000",
  "send_email": "has(args.recipient) && contains(args.recipient, '@')",
  "execute_query": "args.query.startsWith('SELECT') && !contains(args.query, 'DROP')"
}
```

### Example: Protecting a task management system
```bash
# Create constraints file
echo '{
  "add_task": "size(args.description) > 5",
  "mark_task_done": "has(args.finalUpdate) && size(args.finalUpdate) > 0"
}' > task_constraints.json

# Run with constraints
go run . task_constraints.json ../tasks_mcp/tasks_mcp
```

## Constraint Evaluation

- **Empty constraint**: Tool call allowed (no constraint = no restriction)
- **Constraint returns true**: Tool call forwarded to upstream server
- **Constraint returns false**: Tool call rejected with error message
- **Constraint evaluation error**: Tool call rejected with error details

## Features

- **CEL Expression Engine**: Powerful, safe expression evaluation
- **Per-tool Constraints**: Different rules for different tools
- **Argument Validation**: Access to all tool call parameters
- **Transparent Proxying**: Non-constrained tools work normally
- **Error Handling**: Clear error messages for constraint violations
- **Hot Reloading**: Restart server to reload constraint changes

## Use Cases

- **Security**: Prevent dangerous operations based on parameters
- **Business Rules**: Enforce domain-specific validation logic  
- **Rate Limiting**: Control tool usage patterns
- **Data Validation**: Ensure argument quality and format
- **Access Control**: Restrict tool usage based on context
- **Development Safety**: Prevent accidental destructive operations

## CEL Expression Examples

```javascript
// String validation
size(args.name) >= 3 && size(args.name) <= 50

// Numeric ranges  
args.amount > 0 && args.amount <= 1000

// Pattern matching
matches(args.email, r'[^@]+@[^@]+\.[^@]+')

// List operations
args.tags.size() <= 5 && args.tags.all(tag, size(tag) > 0)

// Complex conditions
has(args.config) && args.config.enabled == true && args.config.level in ['info', 'warn', 'error']
```