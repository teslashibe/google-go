# google-go

Multi-account Google client (Gmail + Calendar + Drive + Docs) for Go, with MCP tool exposure via [`mcptool`](https://github.com/teslashibe/mcptool).

Manage N Google accounts (personal, work, side-project, …) from a single `*Manager` instance. Every MCP tool takes an `account` parameter (alias or email) so agents can operate across all your inboxes and calendars seamlessly.

## Features

- **Multi-account** — single Manager, unlimited accounts via aliases
- **Gmail API v1** — search, read, send, reply, forward, draft, labels, archive, trash
- **Calendar API v3** — list events, create, update, delete, free/busy, RSVP
- **Drive API v3** — list/search files, read/export, create, update, delete
- **Drive comments** — add and list document-level comments on Drive files and Google Docs
- **Docs API v1** — structured, revision-safe text replacement without wholesale document conversion
- **Unified views** — cross-account inbox search and merged calendar agenda
- **OAuth2 auto-refresh** — handles token lifecycle transparently
- **MCP tools** — `mcp/` subpackage implements `mcptool.Provider` (zero-drift schemas)

## Install

Use Go 1.25.13 or newer on a currently supported patched release.

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
account, err := mgr.Authenticate(ctx, "work", "you@company.com")
if err != nil {
    log.Fatal(err)
}
log.Printf("authenticated %s", account.Email)

// Search across one account
results, err := mgr.SearchEmail(ctx, "work", "in:inbox is:unread", 20, "")
if err != nil {
	log.Fatal(err)
}

// Unified agenda across all accounts
events, err := mgr.UnifiedAgenda(ctx, timeMin, timeMax, 100)
if err != nil {
	log.Fatal(err)
}
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
- `google_drive_*` — drive file operations (list/read/create/update/delete)
- `google_docs_*` — structured Docs edits and comment-based change proposals

`google_docs_propose_change_comment` records a proposed replacement as a
document-level comment. It is intentionally not described as a native Google
Docs suggestion: suggestion-mode writes require Google Developer Preview
access and are not available through the stable client used by this module.

`google_docs_replace_text` directly edits a document through the Docs API.
Call `google_docs_get_metadata` first, then pass its `revision_id` as
`required_revision_id` when coordinating with other editors so the write fails
instead of applying against a stale document revision.

## Configuration

Credentials stored in `~/.google-mcp/`:

The Docs API scope is included in the default OAuth scopes. Accounts
authenticated before Docs support was added may need to be authenticated again
to grant the new scope.

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
