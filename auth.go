package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

const authTimeout = 3 * time.Minute

type oauthClientSecretFile struct {
	Installed *oauthClientSecret `json:"installed"`
	Web       *oauthClientSecret `json:"web"`
}

type oauthClientSecret struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURI      string   `json:"auth_uri"`
	TokenURI     string   `json:"token_uri"`
	RedirectURIs []string `json:"redirect_uris"`
}

func loadOAuth2Config(keysPath string, scopes []string) (*oauth2.Config, error) {
	b, err := os.ReadFile(keysPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrOAuthKeysNotFound
		}
		return nil, fmt.Errorf("read oauth keys: %w", err)
	}

	var parsed oauthClientSecretFile
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOAuthKeys, err)
	}

	cfg := parsed.Installed
	if cfg == nil {
		cfg = parsed.Web
	}
	if cfg == nil || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, ErrInvalidOAuthKeys
	}

	endpoint := oauth2.Endpoint{
		AuthURL:  firstNonEmpty(cfg.AuthURI, googleoauth.Endpoint.AuthURL),
		TokenURL: firstNonEmpty(cfg.TokenURI, googleoauth.Endpoint.TokenURL),
	}

	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     endpoint,
		Scopes:       scopes,
		RedirectURL:  firstNonEmpty(cfg.RedirectURIs...),
	}, nil
}

func runOAuthBrowserFlow(ctx context.Context, baseCfg *oauth2.Config) (*oauth2.Token, error) {
	if baseCfg == nil {
		return nil, ErrInvalidOAuthKeys
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start auth listener: %w", err)
	}
	defer ln.Close()

	authCfg := *baseCfg
	authCfg.RedirectURL = fmt.Sprintf("http://%s/oauth2callback", ln.Addr().String())

	state, err := randomState(18)
	if err != nil {
		return nil, fmt.Errorf("generate oauth state: %w", err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "invalid oauth state", http.StatusBadRequest)
			select {
			case errCh <- errors.New("oauth state mismatch"):
			default:
			}
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			http.Error(w, "oauth denied", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("oauth denied: %s", e):
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing oauth code", http.StatusBadRequest)
			select {
			case errCh <- errors.New("oauth callback missing code"):
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h3>Google account connected.</h3>You can close this tab.</body></html>"))
		select {
		case codeCh <- code:
		default:
		}
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- fmt.Errorf("oauth callback server failed: %w", serveErr):
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := authCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("open browser failed (%v). open this URL manually: %s", err, authURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		return nil, fmt.Errorf("oauth flow timed out: %w", waitCtx.Err())
	case err := <-errCh:
		return nil, err
	case code := <-codeCh:
		tok, err := authCfg.Exchange(waitCtx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange oauth code: %w", err)
		}
		return tok, nil
	}
}

func randomState(length int) (string, error) {
	if length <= 0 {
		length = 12
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
