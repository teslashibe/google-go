# agent-setup integration snippet

Use this snippet in `agent-setup/backend/internal/mcp/platforms/platforms.go` to wire `google-go` into the host registry.

```go
import (
    "context"
    "encoding/json"

    google "github.com/teslashibe/google-go"
    googlemcp "github.com/teslashibe/google-go/mcp"
)

func Google() Plugin {
    const platform = "google"
    return Plugin{
        Binding: mcp.PlatformBinding{
            Provider: googlemcp.Provider{},
            NewClient: func(_ context.Context, raw json.RawMessage) (any, error) {
                // The host stores per-user credentials under ~/.google-mcp/.
                // Optionally override by passing {"config_path":"/path/to/config.json"}.
                var in struct {
                    ConfigPath string `json:"config_path,omitempty"`
                }
                _ = json.Unmarshal(raw, &in)
                return google.NewManager(in.ConfigPath)
            },
        },
        Validator: simpleValidator{platform: platform, requireOneOf: []string{"config_path"}},
    }
}
```

Notes:

- If `config_path` is omitted, `google-go` defaults to `~/.google-mcp/config.json`.
- Tool names are `google_gmail_*`, `google_calendar_*`, `google_drive_*`, and `google_docs_*`.
- Accounts authenticated before Docs support was added may need to be authenticated again to grant the Docs API scope.
- Docs change proposals are document-level comments, not native suggestion-mode edits.
- The manager supports N accounts and unified cross-account tools out of the box.
