# ADK Agent Runner MCP Tool

An MCP server that analyzes ADK (Agent Deployment Kit) Python scripts to extract agent and sub-agent relationships.

## Tools Provided

- `adk_graph` - Extracts agent and sub-agent relationships from an ADK Python script

## Getting Started

Run the ADK agent analysis server:

```bash
go run .
```

You can also customize the tool name and description:

```bash
go run . -toolName="analyze_adk_agents" -toolDescription="Custom ADK agent analyzer"
```

## Usage Examples

### Analyzing ADK Python code
```json
{
  "name": "adk_graph",
  "arguments": {
    "source": "from adk import Agent, TaskAgent\n\n# Main orchestrator\nmain_agent = Agent(name='MainOrchestrator')\n\n# Task processing agents\ntask_agent = TaskAgent(name='TaskProcessor')\ndata_agent = Agent(name='DataHandler')\n\n# Connect agents\nmain_agent.add_subagent(task_agent)\nmain_agent.add_subagent(data_agent)"
  }
}
```

## Response Format

The tool analyzes the Python AST and returns a JSON structure showing:

- Root agents and their relationships
- Sub-agent hierarchies  
- Agent types and names
- Connection patterns

Example response:
```json
{
  "root_agent": "MainOrchestrator",
  "subagents": {
    "TaskProcessor": "TaskAgent",
    "DataHandler": "Agent"
  },
  "connections": [
    {"from": "MainOrchestrator", "to": "TaskProcessor"},
    {"from": "MainOrchestrator", "to": "DataHandler"}
  ]
}
```

## Features

- Python AST parsing for accurate code analysis
- Detection of various Agent types (Agent, TaskAgent, etc.)
- Extraction of agent hierarchies and relationships
- Support for complex ADK Python scripts
- Detailed JSON output with agent graph structure

## Use Cases

- Visualizing ADK agent architectures
- Understanding agent relationships in complex systems
- Documenting ADK application structures
- Debugging agent connection issues
- Planning agent deployment strategies

## Requirements

- Python interpreter must be available in PATH
- Temporary directory access for script execution