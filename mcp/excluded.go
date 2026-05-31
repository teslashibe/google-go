package mcp

// Excluded are exported Manager methods intentionally not exposed directly
// as standalone MCP tools.
var Excluded = map[string]string{
	"Resolve":      "internal helper for tool handlers to resolve account alias/email",
	"GetDriveFile": "file-metadata fetch used by sync/conflict-detection callers; not a standalone agent tool (google_drive_* read tools cover agent use)",
}
