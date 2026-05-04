package mcp

import "github.com/teslashibe/mcptool"

// Provider exposes Google Gmail + Calendar tools.
type Provider struct{}

// Platform returns the platform key used by host registries.
func (Provider) Platform() string { return "google" }

// Tools returns all google_* MCP tools.
func (Provider) Tools() []mcptool.Tool {
	out := make([]mcptool.Tool, 0, len(accountTools)+len(gmailTools)+len(calendarTools)+len(driveTools)+len(unifiedTools))
	out = append(out, accountTools...)
	out = append(out, gmailTools...)
	out = append(out, calendarTools...)
	out = append(out, driveTools...)
	out = append(out, unifiedTools...)
	return out
}
