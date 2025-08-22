# SQLite MCP Tool

An MCP server that provides SQLite database operations with session management for secure, isolated database interactions.

## Tools Provided

- `create_db` - Create a new SQLite database session
- `run_sql` - Execute SQL statements against a session database

## Getting Started

Run the SQLite MCP server:

```bash
go run ./cmd/sqlite_mcp
```

With custom data directory:

```bash
go run ./cmd/sqlite_mcp -data-dir=/path/to/data
```

## Command Line Options

- `-data-dir` - Directory to store database files (default: "/tmp/sqlite_mcp")

## Usage Examples

### Creating a database session
```json
{
  "name": "create_db",
  "arguments": {}
}
```

Response:
```json
{
  "session_id": "db-session-12345"
}
```

### Creating tables and inserting data
```json
{
  "name": "run_sql",
  "arguments": {
    "session": "db-session-12345",
    "sql": "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)"
  }
}
```

```json
{
  "name": "run_sql",
  "arguments": {
    "session": "db-session-12345", 
    "sql": "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')"
  }
}
```

### Querying data
```json
{
  "name": "run_sql",
  "arguments": {
    "session": "db-session-12345",
    "sql": "SELECT * FROM users WHERE name LIKE 'A%'"
  }
}
```

### Complex operations
```json
{
  "name": "run_sql",
  "arguments": {
    "session": "db-session-12345",
    "sql": "UPDATE users SET email = 'newemail@example.com' WHERE id = 1"
  }
}
```

## Session Management

- **Isolated Sessions**: Each session gets its own SQLite database file
- **Automatic Cleanup**: Sessions expire after 15 minutes of inactivity
- **Session Persistence**: Database files persist until session timeout
- **Unique Identifiers**: Each session has a unique ID for security

## Security Features

- **Session Isolation**: No cross-session data access
- **Single Statement**: Only one SQL statement per call for safety
- **Temporary Storage**: Sessions are cleaned up automatically
- **No Direct File Access**: All operations go through session management

## Supported SQL Operations

All standard SQLite operations are supported:

- **DDL**: `CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`, etc.
- **DML**: `INSERT`, `UPDATE`, `DELETE`, `SELECT`
- **Indexes**: `CREATE INDEX`, `DROP INDEX`
- **Views**: `CREATE VIEW`, `DROP VIEW`  
- **Transactions**: `BEGIN`, `COMMIT`, `ROLLBACK`
- **Pragmas**: `PRAGMA` statements for configuration

## Response Format

Successful operations return:
```json
{
  "success": true,
  "rows_affected": 3,
  "results": [
    {"id": 1, "name": "Alice", "email": "alice@example.com"}
  ]
}
```

Error responses include:
```json
{
  "success": false,
  "error": "SQL error description"
}
```

## Use Cases

- **Prototyping**: Quick database experimentation
- **Testing**: Isolated test databases for development
- **Data Analysis**: Ad-hoc querying and analysis
- **Temporary Storage**: Session-based data persistence
- **Learning**: Safe environment for SQL practice
- **Integration Testing**: Database operations in test suites

## Limitations

- Sessions expire after 15 minutes of inactivity
- One SQL statement per call for security
- SQLite limitations apply (no concurrent writes, etc.)
- Database files stored in temporary directory structure