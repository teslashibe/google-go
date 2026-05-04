package google

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	defaultConfigDirName = ".google-mcp"
	configFileName       = "config.json"
	oauthKeysFileName    = "oauth-keys.json"
	accountsDirName      = "accounts"
	tokenFileName        = "token.json"
)

// Config stores account aliases and optional defaults.
type Config struct {
	DefaultAccount string                   `json:"default_account,omitempty"`
	Accounts       map[string]AccountConfig `json:"accounts,omitempty"`
}

// AccountConfig stores one alias mapping and token file location.
type AccountConfig struct {
	Email     string `json:"email"`
	TokenPath string `json:"token_path,omitempty"`
}

// DefaultConfigPath returns ~/.google-mcp/config.json.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(defaultConfigDirName, configFileName)
	}
	return filepath.Join(home, defaultConfigDirName, configFileName)
}

func configDirFromPath(configPath string) string {
	return filepath.Dir(configPath)
}

func defaultTokenPath(configDir, alias string) string {
	if alias == "" {
		alias = "default"
	}
	return filepath.Join(configDir, accountsDirName, alias, tokenFileName)
}

func resolveTokenPath(configDir, alias, configured string) string {
	if configured == "" {
		return defaultTokenPath(configDir, alias)
	}
	if filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(configDir, configured)
}

func relativeTokenPath(configDir, tokenPath string) string {
	rel, err := filepath.Rel(configDir, tokenPath)
	if err != nil {
		return tokenPath
	}
	if rel == "." {
		return tokenPath
	}
	return rel
}

type fileConfigStore struct {
	mu   sync.Mutex
	path string
}

func newFileConfigStore(path string) *fileConfigStore {
	return &fileConfigStore{path: filepath.Clean(path)}
}

func (s *fileConfigStore) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	configDir := configDirFromPath(s.path)
	if err := ensureConfigLayout(configDir); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := readJSONFile(s.path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = Config{Accounts: map[string]AccountConfig{}}
			if err := writeJSONFile(s.path, cfg, 0o600); err != nil {
				return Config{}, err
			}
			return cfg, nil
		}
		return Config{}, err
	}
	if cfg.Accounts == nil {
		cfg.Accounts = map[string]AccountConfig{}
	}
	return cfg, nil
}

func (s *fileConfigStore) Save(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.Accounts == nil {
		cfg.Accounts = map[string]AccountConfig{}
	}
	configDir := configDirFromPath(s.path)
	if err := ensureConfigLayout(configDir); err != nil {
		return err
	}
	return writeJSONFile(s.path, cfg, 0o600)
}

func ensureConfigLayout(configDir string) error {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(configDir, accountsDirName), 0o700); err != nil {
		return fmt.Errorf("create accounts dir: %w", err)
	}
	return nil
}

func writeJSONFile(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode json: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func readJSONFile(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
