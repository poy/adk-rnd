# Human in the Loop Wrapper

This wraps a stdio MCP tool in a way that will host a server locally on port
8080 that will force each tool call to have a human hit approve.

## Instructions

You need a underlying MCP server, consider using the ../tasks MCP tool.

Then you can simply run it with `go run`:

```bash
go run . tasks-mcp
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
