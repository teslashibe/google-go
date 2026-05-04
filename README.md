# google-go

Multi-account Google client (Gmail + Calendar) for Go, with MCP tool exposure via [`mcptool`](https://github.com/teslashibe/mcptool).

Manage N Google accounts (personal, work, side-project, …) from a single `*Manager` instance. Every MCP tool takes an `account` parameter (alias or email) so agents can operate across all your inboxes and calendars seamlessly.

## Features

- **Multi-account** — single Manager, unlimited accounts via aliases
- **Gmail API v1** — search, read, send, reply, forward, draft, labels, archive, trash
- **Calendar API v3** — list events, create, update, delete, free/busy, RSVP
- **Unified views** — cross-account inbox search and merged calendar agenda
- **OAuth2 auto-refresh** — handles token lifecycle transparently
- **MCP tools** — `mcp/` subpackage implements `mcptool.Provider` (zero-drift schemas)

## Install

```sh
go get github.com/teslashibe/google-go
```

## Quick Start

```go
mgr, err := google.NewManager(google.DefaultConfigPath())
if err != nil {
    log.Fatal(err)
}

// Authenticate a new account (opens browser for OAuth consent)
if err := mgr.Authenticate(ctx, "work", "you@company.com"); err != nil {
    log.Fatal(err)
}

// Search across one account
results, err := mgr.SearchEmail(ctx, "work", "in:inbox is:unread")

// Unified agenda across all accounts
events, err := mgr.UnifiedAgenda(ctx, timeMin, timeMax)
```

## MCP Usage

```go
import (
    "github.com/teslashibe/mcptool"
    google "github.com/teslashibe/google-go"
    googlemcp "github.com/teslashibe/google-go/mcp"
)

mgr, _ := google.NewManager(google.DefaultConfigPath())
for _, tool := range (googlemcp.Provider{}).Tools() {
    // register with your MCP server, passing mgr as the client
}
```

## Tool Naming

All tools are prefixed by service:

- `google_gmail_*` — email operations (search, send, labels, etc.)
- `google_calendar_*` — calendar operations (events, free/busy, RSVP, etc.)

## Configuration

Credentials stored in `~/.google-mcp/`:

```
~/.google-mcp/
├── config.json               # Account aliases and settings
├── oauth-keys.json           # Google OAuth app credentials (Desktop type)
└── accounts/
    ├── work/
    │   └── token.json        # OAuth2 refresh token
    └── personal/
        └── token.json
```

## License

MIT
