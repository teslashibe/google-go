package google

import "errors"

var (
	// ErrAccountNotFound is returned when an alias/email does not exist.
	ErrAccountNotFound = errors.New("google: account not found")
	// ErrAccountNotAuthenticated is returned when an account exists but has no valid token.
	ErrAccountNotAuthenticated = errors.New("google: account not authenticated")
	// ErrOAuthKeysNotFound indicates ~/.google-mcp/oauth-keys.json is missing.
	ErrOAuthKeysNotFound = errors.New("google: oauth keys file not found")
	// ErrInvalidOAuthKeys indicates oauth-keys.json is malformed/incomplete.
	ErrInvalidOAuthKeys = errors.New("google: invalid oauth keys")
	// ErrInvalidInput indicates invalid tool or method input.
	ErrInvalidInput = errors.New("google: invalid input")
)
