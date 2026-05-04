package mcp_test

import (
	"reflect"
	"strings"
	"testing"

	google "github.com/teslashibe/google-go"
	googlemcp "github.com/teslashibe/google-go/mcp"
	"github.com/teslashibe/mcptool"
)

func TestEveryManagerMethodIsWrappedOrExcluded(t *testing.T) {
	rep := mcptool.Coverage(
		reflect.TypeOf(&google.Manager{}),
		(googlemcp.Provider{}).Tools(),
		googlemcp.Excluded,
	)
	if len(rep.Missing) > 0 {
		t.Fatalf("methods missing MCP exposure (wrap or exclude): %v", rep.Missing)
	}
	if len(rep.UnknownExclusions) > 0 {
		t.Fatalf("excluded.go references missing methods: %v", rep.UnknownExclusions)
	}
	if len(rep.Wrapped)+len(rep.Excluded) == 0 {
		t.Fatal("no wrapped or excluded methods found")
	}
}

func TestToolsValidate(t *testing.T) {
	if err := mcptool.ValidateTools((googlemcp.Provider{}).Tools()); err != nil {
		t.Fatal(err)
	}
}

func TestPlatformName(t *testing.T) {
	if got := (googlemcp.Provider{}).Platform(); got != "google" {
		t.Errorf("Platform() = %q, want google", got)
	}
}

func TestToolsHaveGooglePrefix(t *testing.T) {
	for _, tool := range (googlemcp.Provider{}).Tools() {
		if !strings.HasPrefix(tool.Name, "google_") {
			t.Errorf("tool %q lacks google_ prefix", tool.Name)
		}
	}
}
