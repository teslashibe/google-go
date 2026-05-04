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
- Tool names are `google_gmail_*` and `google_calendar_*`.
- The manager supports N accounts and unified cross-account tools out of the box.
