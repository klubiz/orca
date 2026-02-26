// Package agent implements the Claude-based agent runtime for Orca.
// It handles Claude API interaction, tool definitions, and agent lifecycle management.
package agent

// ToolDef describes a tool an agent can use.
type ToolDef struct {
	Name        string
	Description string
}

// AvailableTools returns the built-in tool definitions.
// These are conceptual tool definitions for the agent system.
// In a real implementation, these would map to actual Claude tool_use definitions.
var AvailableTools = map[string]ToolDef{
	"read_file":   {Name: "read_file", Description: "Read a file from disk"},
	"write_file":  {Name: "write_file", Description: "Write content to a file"},
	"run_command": {Name: "run_command", Description: "Execute a shell command"},
	"search_code": {Name: "search_code", Description: "Search for patterns in code"},
	"list_files":  {Name: "list_files", Description: "List files in a directory"},
}
