// Package google provides a multi-account Google client covering Gmail and
// Google Calendar, with native support for N accounts via aliases.
//
// The primary entry point is [Manager], which holds authenticated [Client]
// instances keyed by alias or email address. Each Client wraps the official
// Google Gmail v1 and Calendar v3 APIs with OAuth2 token auto-refresh.
//
// The mcp/ subpackage exposes the full surface as MCP tools via
// [mcptool.Provider], following the same pattern as x-go, linkedin-go, etc.
// Tool names use the google_gmail_ and google_calendar_ prefixes.
package google
