# Human in the Loop Wrapper

This wraps a stdio MCP tool in a way that will host a server locally on port
8080 that will force each tool call to have a human hit approve.

## Instructions

You need a underlying MCP server, consider using the ../tasks MCP tool.

Then you can simply run it with `go run`:

```bash
go run . tasks-mcp
```

If you also want to constrain a method, you can add a constraints (JSON) file. This is a `map[string]string` encoded where the key is the method name (e.g., `add_task`) and the value is a Google Common Expression Langauge expression. The expression must return a boolean. The arguments passed into the tool are avilable under `args`. For example:

```
args.description.size() > 3
```

This ensures the field `description` has a size greater than 3.

Therefore, an entire file would look like the following:

```json
{
  "add_task": "args.description.size() > 3"
}
```

# Human-in-the-Loop Wrapper for MCP Tools

This tool wraps a standard **stdio-based MCP server** and hosts a local web UI (on port `8080`) that **requires human approval for every tool call**.

It's useful when you want to gate tool invocations with explicit manual review.

---

## Getting Started

You'll need an underlying MCP server. For example, you can use the `../tasks` MCP tool.

Run the wrapper like this:

```bash
go run . tasks-mcp
```

This will:

Launch the MCP wrapper server

Host a simple approval UI at http://localhost:8080

Intercept and queue tool calls for manual approval

## Adding Constraints (Optional)
You can define constraints per method to automatically reject invalid calls before they reach the approval UI.

Create a JSON file that maps method names to CEL (Common Expression Language) expressions. Each expression must return a boolean and can access arguments via the args variable.

### Example Constraint
```
{
  "add_task": "args.description.size() > 3"
}
```

This constraint ensures that the description field of the add_task method has a length greater than 3.

## Tips
If a constraint fails, the tool call is immediately rejected.

The UI will only show calls that pass constraint checks (if any).
