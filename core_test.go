package google

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// Keep the README quick-start calls aligned with the exported API.
var (
	_ func(*Manager, context.Context, string, string) (AccountInfo, error)                     = (*Manager).Authenticate
	_ func(*Manager, context.Context, string, string, int, string) (SearchEmailsResult, error) = (*Manager).SearchEmail
	_ func(*Manager, context.Context, time.Time, time.Time, int) ([]AccountEvent, error)       = (*Manager).UnifiedAgenda
)

func TestConfigPaths(t *testing.T) {
	configDir := t.TempDir()

	if got, want := defaultTokenPath(configDir, "work"), filepath.Join(configDir, "accounts", "work", "token.json"); got != want {
		t.Fatalf("defaultTokenPath() = %q, want %q", got, want)
	}
	if got, want := defaultTokenPath(configDir, ""), filepath.Join(configDir, "accounts", "default", "token.json"); got != want {
		t.Fatalf("defaultTokenPath(empty alias) = %q, want %q", got, want)
	}

	relative := filepath.Join("accounts", "work", "custom.json")
	if got, want := resolveTokenPath(configDir, "work", relative), filepath.Join(configDir, relative); got != want {
		t.Fatalf("resolveTokenPath(relative) = %q, want %q", got, want)
	}
	absolute := filepath.Join(t.TempDir(), "token.json")
	if got := resolveTokenPath(configDir, "work", absolute); got != absolute {
		t.Fatalf("resolveTokenPath(absolute) = %q, want %q", got, absolute)
	}
	if got := relativeTokenPath(configDir, filepath.Join(configDir, relative)); got != relative {
		t.Fatalf("relativeTokenPath() = %q, want %q", got, relative)
	}
}

func TestFileConfigStoreRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"accounts":`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := newFileConfigStore(path).Load()
	if err == nil || !strings.Contains(err.Error(), "decode ") {
		t.Fatalf("Load() error = %v, want decode error", err)
	}
}

func TestLoadOAuth2Config(t *testing.T) {
	t.Run("installed client and explicit endpoints", func(t *testing.T) {
		path := writeTestFile(t, `{
			"installed": {
				"client_id": "client-id",
				"client_secret": "client-secret",
				"auth_uri": "https://auth.example.test",
				"token_uri": "https://token.example.test",
				"redirect_uris": ["http://localhost/first", "http://localhost/second"]
			}
		}`)
		scopes := []string{"scope-a", "scope-b"}

		cfg, err := loadOAuth2Config(path, scopes)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ClientID != "client-id" || cfg.ClientSecret != "client-secret" {
			t.Fatalf("unexpected client credentials: %#v", cfg)
		}
		if cfg.Endpoint.AuthURL != "https://auth.example.test" || cfg.Endpoint.TokenURL != "https://token.example.test" {
			t.Fatalf("unexpected endpoint: %#v", cfg.Endpoint)
		}
		if cfg.RedirectURL != "http://localhost/first" {
			t.Fatalf("RedirectURL = %q, want first configured URI", cfg.RedirectURL)
		}
		if !reflect.DeepEqual(cfg.Scopes, scopes) {
			t.Fatalf("Scopes = %#v, want %#v", cfg.Scopes, scopes)
		}
	})

	t.Run("web client and default endpoints", func(t *testing.T) {
		path := writeTestFile(t, `{"web":{"client_id":"web-id","client_secret":"web-secret"}}`)
		cfg, err := loadOAuth2Config(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ClientID != "web-id" {
			t.Fatalf("ClientID = %q, want web-id", cfg.ClientID)
		}
		if cfg.Endpoint.AuthURL != googleoauth.Endpoint.AuthURL || cfg.Endpoint.TokenURL != googleoauth.Endpoint.TokenURL {
			t.Fatalf("Endpoint = %#v, want Google auth and token URLs", cfg.Endpoint)
		}
	})

	for name, content := range map[string]string{
		"malformed":      `{`,
		"missing client": `{"installed":{"client_id":"","client_secret":"secret"}}`,
		"missing type":   `{"client_id":"id","client_secret":"secret"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadOAuth2Config(writeTestFile(t, content), nil)
			if !errors.Is(err, ErrInvalidOAuthKeys) {
				t.Fatalf("error = %v, want ErrInvalidOAuthKeys", err)
			}
		})
	}

	_, err := loadOAuth2Config(filepath.Join(t.TempDir(), "missing.json"), nil)
	if !errors.Is(err, ErrOAuthKeysNotFound) {
		t.Fatalf("missing file error = %v, want ErrOAuthKeysNotFound", err)
	}
}

func TestTokenPersistenceAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "token.json")
	want := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := saveToken(path, want); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token permissions = %o, want 600", got)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := parent.Mode().Perm(); got != 0o700 {
		t.Fatalf("token directory permissions = %o, want 700", got)
	}

	got, err := loadToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadToken() = %#v, want %#v", got, want)
	}

	if err := saveToken(path, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("saveToken(nil) error = %v, want ErrInvalidInput", err)
	}
	empty := writeTestFile(t, `{}`)
	if _, err := loadToken(empty); err == nil {
		t.Fatal("loadToken(empty token) succeeded")
	}
}

func TestPersistingTokenSourceWritesChangedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	old := &oauth2.Token{AccessToken: "old"}
	next := &oauth2.Token{AccessToken: "new", RefreshToken: "refresh"}
	source := &persistingTokenSource{
		tokenSource: oauth2.StaticTokenSource(next),
		tokenPath:   path,
		last:        cloneToken(old),
	}

	got, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new" {
		t.Fatalf("Token() access token = %q, want new", got.AccessToken)
	}
	persisted, err := loadToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.AccessToken != "new" || persisted.RefreshToken != "refresh" {
		t.Fatalf("persisted token = %#v", persisted)
	}
}

func TestReadLimitedBounds(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		max       int
		want      string
		truncated bool
	}{
		{name: "below", input: "abc", max: 4, want: "abc"},
		{name: "at bound", input: "abcd", max: 4, want: "abcd"},
		{name: "over bound", input: "abcde", max: 4, want: "abcd", truncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated, err := readLimited(strings.NewReader(tt.input), tt.max)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want || truncated != tt.truncated {
				t.Fatalf("readLimited() = (%q, %v), want (%q, %v)", got, truncated, tt.want, tt.truncated)
			}
		})
	}
}

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
