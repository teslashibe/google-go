package mcp

// Excluded are exported Manager methods intentionally not exposed directly
// as standalone MCP tools.
var Excluded = map[string]string{
	"Resolve": "internal helper for tool handlers to resolve account alias/email",
}
