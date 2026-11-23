package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	google "github.com/teslashibe/google-go"
	googlemcp "github.com/teslashibe/google-go/mcp"
	"github.com/teslashibe/mcptool"
	"golang.org/x/oauth2"
	calendarapi "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
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

func TestDocsAndCommentToolsAreRegistered(t *testing.T) {
	wanted := map[string]bool{
		"google_drive_comment_create":        false,
		"google_drive_comments_list":         false,
		"google_docs_get_metadata":           false,
		"google_docs_replace_text":           false,
		"google_docs_propose_change_comment": false,
	}
	for _, tool := range (googlemcp.Provider{}).Tools() {
		if _, ok := wanted[tool.Name]; ok {
			wanted[tool.Name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("tool %q is not registered", name)
		}
	}
}

func TestDefaultScopesIncludeDocs(t *testing.T) {
	const docsScope = "https://www.googleapis.com/auth/documents"
	for _, scope := range google.DefaultGoogleScopes() {
		if scope == docsScope {
			return
		}
	}
	t.Errorf("default OAuth scopes do not include %q", docsScope)
}

// calendarServer serves the Calendar API paths the event tools call and
// records each request's method, query and decoded JSON body.
type calendarRequest struct {
	method string
	path   string
	query  url.Values
	body   map[string]any
}

func newCalendarTestManager(t *testing.T, respond func(r calendarRequest) string) (*google.Manager, *[]calendarRequest) {
	t.Helper()
	var seen []calendarRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := calendarRequest{method: r.Method, path: r.URL.Path, query: r.URL.Query()}
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &req.body); err != nil {
					t.Errorf("request body is not JSON: %v", err)
				}
			}
		}
		seen = append(seen, req)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, respond(req))
	}))
	t.Cleanup(server.Close)
	m, err := google.NewManagerFromTokenSources(context.Background(), []google.AccountSource{{
		Alias:       "work",
		Email:       "reader@example.com",
		TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-access", TokenType: "Bearer"}),
	}}, option.WithEndpoint(server.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	return m, &seen
}

func calendarTool(t *testing.T, name string) mcptool.Tool {
	t.Helper()
	for _, tool := range (googlemcp.Provider{}).Tools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q is not registered", name)
	return mcptool.Tool{}
}

func invoke(t *testing.T, m *google.Manager, name, input string) (any, error) {
	t.Helper()
	return calendarTool(t, name).Invoke(context.Background(), m, json.RawMessage(input))
}

func eventEcho(r calendarRequest) string {
	out := map[string]any{"id": "e1"}
	for k, v := range r.body {
		out[k] = v
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

func TestCalendarSchemasNameNewFields(t *testing.T) {
	for name, fields := range map[string][]string{
		"google_calendar_create_event": {"transparency", "visibility", "private_properties"},
		"google_calendar_update_event": {"transparency", "visibility", "private_properties"},
		"google_calendar_events":       {"private_property"},
	} {
		props, _ := calendarTool(t, name).InputSchema["properties"].(map[string]any)
		for _, field := range fields {
			if _, ok := props[field]; !ok {
				t.Errorf("%s schema lacks %q", name, field)
			}
		}
	}
	props := calendarTool(t, "google_calendar_create_event").InputSchema["properties"].(map[string]any)
	for field, want := range map[string][]any{
		"transparency": {"opaque", "transparent"},
		"visibility":   {"default", "public", "private"},
	} {
		got, _ := props[field].(map[string]any)["enum"].([]any)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s enum = %v, want %v", field, got, want)
		}
	}
}

func TestCalendarCreateEventTransparency(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_create_event", `{"account":"work","calendar_id":"cal1",
		"event":{"summary":"Busy","start":{"dateTime":"2026-11-24T10:00:00Z"},"end":{"dateTime":"2026-11-24T11:00:00Z"}},
		"transparency":"opaque"}`); err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	if req.method != http.MethodPost || req.path != "/calendars/cal1/events" {
		t.Fatalf("request = %s %s, want POST /calendars/cal1/events", req.method, req.path)
	}
	if req.body["transparency"] != "opaque" || req.body["summary"] != "Busy" {
		t.Fatalf("body = %v, want transparency opaque and summary Busy", req.body)
	}
}

func TestCalendarCreateEventVisibility(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_create_event", `{"account":"work",
		"event":{"summary":"Busy","visibility":"public"},"visibility":"private"}`); err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	if req.path != "/calendars/primary/events" || req.body["visibility"] != "private" {
		t.Fatalf("request = %s body %v, want primary calendar and visibility private (top level wins)", req.path, req.body)
	}
}

func TestCalendarCreateEventPrivateProperties(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	out, err := invoke(t, m, "google_calendar_create_event", `{"account":"work","event":{"summary":"Busy",
		"extendedProperties":{"private":{"kept":"1","camp_sync":"old"},"shared":{"s":"x"}}},
		"private_properties":{"camp_source":"cal1/e9","camp_sync":"sched-1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	ext, _ := req.body["extendedProperties"].(map[string]any)
	private, _ := ext["private"].(map[string]any)
	want := map[string]any{"kept": "1", "camp_source": "cal1/e9", "camp_sync": "sched-1"}
	if !reflect.DeepEqual(private, want) {
		t.Fatalf("private properties = %v, want %v", private, want)
	}
	if shared, _ := ext["shared"].(map[string]any); shared["s"] != "x" {
		t.Fatalf("shared properties = %v, want kept", ext["shared"])
	}
	event, ok := out.(*calendarapi.Event)
	if !ok || event.ExtendedProperties == nil || event.ExtendedProperties.Private["camp_sync"] != "sched-1" {
		t.Fatalf("result = %#v, want the created event with its private properties", out)
	}
}

func TestCalendarCreateEventSendsRemindersOff(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_create_event", `{"account":"work","event":{"summary":"Busy","reminders":{"useDefault":false}}}`); err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	reminders, _ := req.body["reminders"].(map[string]any)
	if v, ok := reminders["useDefault"]; !ok || v != false {
		t.Fatalf("body = %v, want reminders.useDefault false sent", req.body)
	}
}

func TestCalendarUpdateEventTransparency(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_update_event", `{"account":"work","calendar_id":"cal1","event_id":"e1",
		"event":{},"transparency":"transparent"}`); err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	if req.method != http.MethodPatch || req.path != "/calendars/cal1/events/e1" {
		t.Fatalf("request = %s %s, want PATCH /calendars/cal1/events/e1", req.method, req.path)
	}
	if req.body["transparency"] != "transparent" {
		t.Fatalf("body = %v, want transparency transparent", req.body)
	}
}

func TestCalendarUpdateEventVisibility(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_update_event", `{"account":"work","event_id":"e1",
		"event":{},"visibility":"default"}`); err != nil {
		t.Fatal(err)
	}
	if req := onlyRequest(t, *seen); req.body["visibility"] != "default" {
		t.Fatalf("body = %v, want visibility default", req.body)
	}
}

func TestCalendarUpdateEventPrivateProperties(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	if _, err := invoke(t, m, "google_calendar_update_event", `{"account":"work","event_id":"e1",
		"event":{"start":{"dateTime":"2026-11-24T12:00:00Z"}},"private_properties":{"camp_source":"cal1/e9"}}`); err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	ext, _ := req.body["extendedProperties"].(map[string]any)
	if private, _ := ext["private"].(map[string]any); private["camp_source"] != "cal1/e9" {
		t.Fatalf("body = %v, want private camp_source", req.body)
	}
	if _, ok := req.body["start"]; !ok {
		t.Fatalf("body = %v, want the event payload's start kept", req.body)
	}
}

func TestCalendarEventsPrivatePropertyFilter(t *testing.T) {
	m, seen := newCalendarTestManager(t, func(calendarRequest) string {
		return `{"items":[{"id":"b1","summary":"Busy","extendedProperties":{"private":{"camp_source":"cal1/e9","camp_sync":"sched-1"}}}]}`
	})
	out, err := invoke(t, m, "google_calendar_events", `{"account":"work","calendar_id":"cal2",
		"private_property":["camp_sync=sched-1","camp_source=cal1/e9"]}`)
	if err != nil {
		t.Fatal(err)
	}
	req := onlyRequest(t, *seen)
	if req.method != http.MethodGet || req.path != "/calendars/cal2/events" {
		t.Fatalf("request = %s %s, want GET /calendars/cal2/events", req.method, req.path)
	}
	if got, want := req.query["privateExtendedProperty"], []string{"camp_sync=sched-1", "camp_source=cal1/e9"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("privateExtendedProperty = %v, want %v", got, want)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Items []struct {
			ID                 string `json:"id"`
			ExtendedProperties struct {
				Private map[string]string `json:"private"`
			} `json:"extendedProperties"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ExtendedProperties.Private["camp_sync"] != "sched-1" ||
		page.Items[0].ExtendedProperties.Private["camp_source"] != "cal1/e9" {
		t.Fatalf("result = %s, want each event's private properties", raw)
	}
}

func TestCalendarEventsWithoutFilterSendsNone(t *testing.T) {
	m, seen := newCalendarTestManager(t, func(calendarRequest) string { return `{"items":[]}` })
	if _, err := invoke(t, m, "google_calendar_events", `{"account":"work"}`); err != nil {
		t.Fatal(err)
	}
	if req := onlyRequest(t, *seen); req.query.Has("privateExtendedProperty") {
		t.Fatalf("query = %v, want no privateExtendedProperty", req.query)
	}
}

func TestCalendarFieldsRejectBadValuesBeforeGoogle(t *testing.T) {
	for _, tc := range []struct{ tool, input string }{
		{"google_calendar_create_event", `{"account":"work","event":{},"transparency":"busy"}`},
		{"google_calendar_create_event", `{"account":"work","event":{},"visibility":"confidential"}`},
		{"google_calendar_update_event", `{"account":"work","event_id":"e1","event":{},"transparency":"free"}`},
		{"google_calendar_update_event", `{"account":"work","event_id":"e1","event":{},"visibility":"secret"}`},
		{"google_calendar_update_event", `{"account":"work","event_id":"e1","event":{},"private_properties":{"":"x"}}`},
		{"google_calendar_events", `{"account":"work","private_property":["camp_sync"]}`},
		{"google_calendar_events", `{"account":"work","private_property":["=x"]}`},
	} {
		m, seen := newCalendarTestManager(t, eventEcho)
		_, err := invoke(t, m, tc.tool, tc.input)
		var toolErr *mcptool.Error
		if !errors.As(err, &toolErr) || toolErr.Code != "invalid_input" {
			t.Errorf("%s %s: error = %v, want invalid_input", tc.tool, tc.input, err)
		}
		if len(*seen) != 0 {
			t.Errorf("%s %s: called Google %d times, want 0", tc.tool, tc.input, len(*seen))
		}
	}
}

func TestListEventsRejectsMalformedPrivatePropertyOption(t *testing.T) {
	m, seen := newCalendarTestManager(t, eventEcho)
	_, err := m.ListEvents(context.Background(), "work", "", nil, nil, 0, "", google.WithPrivateProperty(" ", "x"))
	if !errors.Is(err, google.ErrInvalidInput) || len(*seen) != 0 {
		t.Fatalf("ListEvents() error = %v after %d calls, want ErrInvalidInput before Google", err, len(*seen))
	}
}

func TestNewManagerFromTokenSourcesRejectsBadAccounts(t *testing.T) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "x"})
	for _, accounts := range [][]google.AccountSource{
		nil,
		{{Alias: "", TokenSource: ts}},
		{{Alias: "work"}},
		{{Alias: "work", TokenSource: ts}, {Alias: "WORK", TokenSource: ts}},
	} {
		if _, err := google.NewManagerFromTokenSources(context.Background(), accounts); !errors.Is(err, google.ErrInvalidInput) {
			t.Errorf("NewManagerFromTokenSources(%v) error = %v, want ErrInvalidInput", accounts, err)
		}
	}
	m, err := google.NewManagerFromTokenSources(context.Background(), []google.AccountSource{
		{Alias: "Work", Email: "Reader@Example.com", TokenSource: ts},
		{Alias: "home", Email: "home@example.com", TokenSource: ts},
	})
	if err != nil {
		t.Fatal(err)
	}
	accounts := m.ListAccounts()
	if len(accounts) != 2 || accounts[1].Alias != "work" || !accounts[1].Authenticated {
		t.Fatalf("ListAccounts() = %#v", accounts)
	}
	if _, err := m.Resolve("reader@example.com"); err != nil {
		t.Fatalf("Resolve by email: %v", err)
	}
	if _, err := m.Resolve(""); err != nil {
		t.Fatalf("Resolve default: %v", err)
	}
}

func onlyRequest(t *testing.T, seen []calendarRequest) calendarRequest {
	t.Helper()
	if len(seen) != 1 {
		t.Fatalf("Google saw %d requests, want 1", len(seen))
	}
	return seen[0]
}
