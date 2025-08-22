# Static JSON MCP Tool

An MCP server that serves paginated data from a JSON file with automatically inferred schema.

## Tools Provided

- `get_data` (default) - Returns paged JSON data with inferred schema

## Getting Started

Run the static JSON server with a required JSON file:

```bash
go run . -file=data.json
```

Customize the tool and server names:

```bash
go run . -file=users.json -tool=get_users -name=UserDataService
```

## Command Line Options

- `-file` - Path to JSON file (required, must contain an array)
- `-tool` - MCP tool name to expose (default: "get_data")  
- `-name` - Name of the MCP server (default: "MockDataTool")

## Usage Examples

### Basic data retrieval
```json
{
  "name": "get_data",
  "arguments": {
    "page": 0,
    "page_size": 10
  }
}
```

### Getting specific page
```json
{
  "name": "get_data",
  "arguments": {
    "page": 2,
    "page_size": 5
  }
}
```

### Using default pagination
```json
{
  "name": "get_data",
  "arguments": {}
}
```

## Response Format

Returns paginated JSON data from the source file. The tool automatically infers the schema from the JSON structure and provides it in the tool's output schema.

## Input File Requirements

- File must contain a valid JSON array
- Array elements should be objects with consistent structure
- File must be readable by the process

Example input file (`data.json`):
```json
[
  {"id": 1, "name": "Alice", "email": "alice@example.com"},
  {"id": 2, "name": "Bob", "email": "bob@example.com"},
  {"id": 3, "name": "Carol", "email": "carol@example.com"}
]
```

## Features

- Automatic JSON schema inference from sample data
- Configurable pagination (default: page 0, page size 10)
- Support for any JSON array structure
- Runtime schema validation
- Customizable tool and server names
- Type-safe field name conversion for Go struct generation

## Use Cases

- Mocking APIs during development
- Serving test data for integration tests
- Prototyping with sample datasets
- Creating read-only data services from static files