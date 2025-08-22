# Golang Run MCP Tool

An MCP server that executes Go code from string input in a secure, isolated environment.

## Tools Provided

- `run_go` - Execute Go source code that contains a main function

## Getting Started

Run the Go execution server:

```bash
go run .
```

You can also customize the tool name and description:

```bash
go run . -toolName="execute_go_code" -toolDescription="Custom Go code runner"
```

## Usage Examples

### Running simple Go code
```json
{
  "name": "run_go",
  "arguments": {
    "source": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello from Go!\")\n}"
  }
}
```

### Running Go code with calculations
```json
{
  "name": "run_go", 
  "arguments": {
    "source": "package main\n\nimport (\n    \"fmt\"\n    \"math\"\n)\n\nfunc main() {\n    result := math.Sqrt(16)\n    fmt.Printf(\"Square root of 16 is: %.2f\\n\", result)\n}"
  }
}
```

## Response Format

The tool returns JSON with the following structure:

```json
{
  "success": true,
  "output": "Hello from Go!",
  "error": ""
}
```

For failed executions:

```json
{
  "success": false,
  "output": "",
  "error": "compilation error details"
}
```

## Features

- Secure execution in temporary directories
- Automatic cleanup of temporary files
- Support for any Go code with a main function
- Detailed error reporting for compilation and runtime errors
- Configurable tool name and description via command-line flags

## Security Notes

- Code is executed in temporary directories that are cleaned up after execution
- Each execution is isolated from others
- Standard Go compiler and runtime security applies