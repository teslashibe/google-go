// Package google provides a multi-account Google client covering Gmail,
// Google Calendar, and Google Drive, with native support for N accounts via
// aliases.
//
// The primary entry point is [Manager], which holds authenticated [Client]
// instances keyed by alias or email address. Each Client wraps the official
// Google Gmail v1, Calendar v3, and Drive v3 APIs with OAuth2 token
// auto-refresh.
//
// The mcp/ subpackage exposes the full surface as MCP tools through the
// mcptool.Provider contract, following the same pattern as x-go, linkedin-go,
// and other integrations.
// Tool names use the google_gmail_, google_calendar_, and google_drive_
// prefixes.
package google
