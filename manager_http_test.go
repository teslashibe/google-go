package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	calendarapi "google.golang.org/api/calendar/v3"
	driveapi "google.golang.org/api/drive/v3"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestGmailReadAPIs(t *testing.T) {
	var requests []string
	encodedBody := base64.RawURLEncoding.EncodeToString([]byte("message body"))
	m := newHTTPTestManager(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			t.Errorf("Gmail method = %s, want GET", r.Method)
		}

		switch r.URL.Path {
		case "/gmail/v1/users/me/messages":
			assertQuery(t, r.URL.Query(), "q", "in:inbox")
			assertQuery(t, r.URL.Query(), "maxResults", "2")
			assertQuery(t, r.URL.Query(), "pageToken", "email-cursor")
			writeJSON(t, w, `{"messages":[{"id":"m1","threadId":"t1"}],"nextPageToken":"email-next"}`)
		case "/gmail/v1/users/me/messages/m1":
			if r.URL.Query().Get("format") == "metadata" {
				writeJSON(t, w, `{
					"id":"m1","threadId":"t1","snippet":"preview","internalDate":"42",
					"labelIds":["INBOX","UNREAD"],
					"payload":{"headers":[
						{"name":"Subject","value":"Status"},
						{"name":"From","value":"sender@example.com"},
						{"name":"Date","value":"Mon"}
					]}
				}`)
				return
			}
			assertQuery(t, r.URL.Query(), "format", "full")
			writeJSON(t, w, fmt.Sprintf(`{
				"id":"m1","threadId":"t1","snippet":"preview",
				"payload":{
					"headers":[
						{"name":"Subject","value":"Status"},
						{"name":"From","value":"sender@example.com"},
						{"name":"To","value":"reader@example.com"}
					],
					"parts":[{"mimeType":"text/plain","body":{"data":"%s"}}]
				}
			}`, encodedBody))
		case "/gmail/v1/users/me/threads":
			assertQuery(t, r.URL.Query(), "q", "newer_than:7d")
			assertQuery(t, r.URL.Query(), "maxResults", "3")
			assertQuery(t, r.URL.Query(), "pageToken", "thread-cursor")
			writeJSON(t, w, `{"threads":[{"id":"t1"}],"nextPageToken":"thread-next"}`)
		case "/gmail/v1/users/me/threads/t1":
			assertQuery(t, r.URL.Query(), "format", "metadata")
			writeJSON(t, w, `{
				"id":"t1",
				"messages":[
					{"id":"m0","internalDate":"10"},
					{"id":"m1","snippet":"latest","internalDate":"42",
					 "payload":{"headers":[{"name":"Subject","value":"Thread status"}]}}
				]
			}`)
		case "/gmail/v1/users/me/labels":
			writeJSON(t, w, `{"labels":[
				{"id":"L2","name":"Zeta","type":"user"},
				{"id":"L1","name":"Alpha","type":"system","messageListVisibility":"show"}
			]}`)
		case "/gmail/v1/users/me/messages/fail":
			http.Error(w, `{"error":{"code":503,"message":"unavailable"}}`, http.StatusServiceUnavailable)
		default:
			t.Errorf("unexpected Gmail request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})

	page, err := m.SearchEmail(context.Background(), "work", "in:inbox", 2, "email-cursor")
	if err != nil {
		t.Fatal(err)
	}
	if page.NextCursor != "email-next" || len(page.Items) != 1 {
		t.Fatalf("SearchEmail() = %#v", page)
	}
	email := page.Items[0]
	if email.ID != "m1" || email.ThreadID != "t1" || email.Subject != "Status" ||
		email.From != "sender@example.com" || email.InternalDate != 42 ||
		len(email.LabelIDs) != 2 {
		t.Fatalf("mapped email = %#v", email)
	}

	full, err := m.ReadEmail(context.Background(), "work", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if full.To != "reader@example.com" || full.Body != "message body" || full.Subject != "Status" {
		t.Fatalf("ReadEmail() = %#v", full)
	}

	threads, err := m.ListThreads(context.Background(), "work", "newer_than:7d", 3, "thread-cursor")
	if err != nil {
		t.Fatal(err)
	}
	if threads.NextCursor != "thread-next" || len(threads.Items) != 1 {
		t.Fatalf("ListThreads() = %#v", threads)
	}
	thread := threads.Items[0]
	if thread.ID != "t1" || thread.MessageCount != 2 || thread.Subject != "Thread status" ||
		thread.Snippet != "latest" || thread.LastInternalDate != 42 {
		t.Fatalf("mapped thread = %#v", thread)
	}

	labels, err := m.ListLabels(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 || labels[0].Name != "Alpha" || labels[0].ID != "L1" ||
		labels[0].MessageListVisibility != "show" || labels[1].Name != "Zeta" {
		t.Fatalf("ListLabels() = %#v", labels)
	}

	if _, err := m.SearchEmail(context.Background(), "work", " ", 1, ""); err != ErrInvalidInput {
		t.Fatalf("blank SearchEmail() error = %v, want ErrInvalidInput", err)
	}
	if _, err := m.ReadEmail(context.Background(), "work", "fail"); err == nil {
		t.Fatal("ReadEmail() succeeded on HTTP 503")
	}
	if len(requests) < 7 {
		t.Fatalf("received %d requests, want all Gmail read paths exercised", len(requests))
	}
}

func TestCalendarReadAPIs(t *testing.T) {
	calendarPage := 0
	var freeBusyBody map[string]any
	m := newHTTPTestManager(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/users/me/calendarList":
			if r.Method != http.MethodGet {
				t.Errorf("calendar list method = %s, want GET", r.Method)
			}
			assertQuery(t, r.URL.Query(), "maxResults", "250")
			calendarPage++
			if calendarPage == 1 {
				if r.URL.Query().Get("pageToken") != "" {
					t.Errorf("first calendar page token = %q", r.URL.Query().Get("pageToken"))
				}
				writeJSON(t, w, `{"items":[{"id":"cal-b","summary":"Beta"}],"nextPageToken":"cal-next"}`)
				return
			}
			assertQuery(t, r.URL.Query(), "pageToken", "cal-next")
			writeJSON(t, w, `{"items":[{"id":"cal-a","summary":"Alpha","primary":true}]}`)
		case "/calendars/cal-a/events":
			if r.Method != http.MethodGet {
				t.Errorf("event list method = %s, want GET", r.Method)
			}
			assertQuery(t, r.URL.Query(), "singleEvents", "true")
			assertQuery(t, r.URL.Query(), "orderBy", "startTime")
			assertQuery(t, r.URL.Query(), "maxResults", "5")
			assertQuery(t, r.URL.Query(), "pageToken", "event-cursor")
			assertQuery(t, r.URL.Query(), "timeMin", "2030-01-02T03:04:05Z")
			assertQuery(t, r.URL.Query(), "timeMax", "2030-01-03T03:04:05Z")
			writeJSON(t, w, `{"items":[{"id":"event-1","summary":"Review"}],"nextPageToken":"event-next"}`)
		case "/calendars/primary/events/event-1":
			if r.Method != http.MethodGet {
				t.Errorf("event get method = %s, want GET", r.Method)
			}
			writeJSON(t, w, `{"id":"event-1","summary":"Review","status":"confirmed"}`)
		case "/freeBusy":
			if r.Method != http.MethodPost {
				t.Errorf("free-busy method = %s, want POST", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&freeBusyBody); err != nil {
				t.Errorf("decode free-busy request: %v", err)
			}
			writeJSON(t, w, `{"calendars":{"cal-a":{"busy":[
				{"start":"2030-01-02T04:00:00Z","end":"2030-01-02T05:00:00Z"}
			]}}}`)
		default:
			t.Errorf("unexpected Calendar request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})

	calendars, err := m.ListCalendars(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if calendarPage != 2 || len(calendars) != 2 || calendars[0].Summary != "Alpha" ||
		!calendars[0].Primary || calendars[1].Summary != "Beta" {
		t.Fatalf("ListCalendars() = %#v after %d pages", calendars, calendarPage)
	}

	start := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events, err := m.ListEvents(context.Background(), "work", "cal-a", &start, &end, 5, "event-cursor")
	if err != nil {
		t.Fatal(err)
	}
	if events.NextCursor != "event-next" || len(events.Items) != 1 ||
		events.Items[0].Id != "event-1" || events.Items[0].Summary != "Review" {
		t.Fatalf("ListEvents() = %#v", events)
	}

	event, err := m.GetEvent(context.Background(), "work", "", "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if event.Id != "event-1" || event.Status != "confirmed" {
		t.Fatalf("GetEvent() = %#v", event)
	}

	busy, err := m.FreeBusy(context.Background(), "work", []string{"cal-a"}, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(busy.Calendars["cal-a"].Busy) != 1 {
		t.Fatalf("FreeBusy() = %#v", busy)
	}
	if freeBusyBody["timeMin"] != start.Format(time.RFC3339) ||
		freeBusyBody["timeMax"] != end.Format(time.RFC3339) {
		t.Fatalf("free-busy body = %#v", freeBusyBody)
	}
	items, ok := freeBusyBody["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("free-busy items = %#v", freeBusyBody["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["id"] != "cal-a" {
		t.Fatalf("free-busy item = %#v", items[0])
	}

	if _, err := m.GetEvent(context.Background(), "work", "primary", " "); err != ErrInvalidInput {
		t.Fatalf("blank GetEvent() error = %v, want ErrInvalidInput", err)
	}
}

func TestDriveReadAPIsAndResponseBounds(t *testing.T) {
	const oversized = "abcdefgh"
	m := newHTTPTestManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Drive method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/files":
			if r.Method != http.MethodGet {
				t.Errorf("Drive list method = %s, want GET", r.Method)
			}
			assertQuery(t, r.URL.Query(), "q", "name contains 'report'")
			assertQuery(t, r.URL.Query(), "pageSize", "2")
			assertQuery(t, r.URL.Query(), "pageToken", "drive-cursor")
			assertQuery(t, r.URL.Query(), "supportsAllDrives", "true")
			assertQuery(t, r.URL.Query(), "includeItemsFromAllDrives", "true")
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, `{
				"files":[{"id":"binary","name":"report.txt","mimeType":"text/plain",
					"modifiedTime":"2030-01-02T03:04:05Z","size":"8","parents":["root"]}],
				"nextPageToken":"drive-next"
			}`)
		case "/files/binary":
			if r.URL.Query().Get("alt") == "media" {
				_, _ = io.WriteString(w, oversized)
				return
			}
			assertQuery(t, r.URL.Query(), "supportsAllDrives", "true")
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, `{"id":"binary","name":"report.txt","mimeType":"text/plain","size":"8"}`)
		case "/files/doc":
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, `{"id":"doc","name":"notes","mimeType":"application/vnd.google-apps.document"}`)
		case "/files/doc/export":
			assertQuery(t, r.URL.Query(), "mimeType", "text/plain")
			_, _ = w.Write([]byte{0xff, 0x00, 0x01})
		case "/files/folder":
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, `{"id":"folder","name":"folder","mimeType":"application/vnd.google-apps.folder"}`)
		default:
			t.Errorf("unexpected Drive request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	})

	page, err := m.ListDriveFiles(context.Background(), "work", "name contains 'report'", 2, "drive-cursor")
	if err != nil {
		t.Fatal(err)
	}
	if page.NextCursor != "drive-next" || len(page.Items) != 1 {
		t.Fatalf("ListDriveFiles() = %#v", page)
	}
	file := page.Items[0]
	if file.ID != "binary" || file.Name != "report.txt" || file.Size != 8 ||
		file.ModifiedTime != "2030-01-02T03:04:05Z" || len(file.Parents) != 1 {
		t.Fatalf("mapped Drive file = %#v", file)
	}

	content, err := m.ReadDriveFile(context.Background(), "work", "binary", "", 4)
	if err != nil {
		t.Fatal(err)
	}
	if content.Content != "abcd" || content.Encoding != "utf-8" || !content.Truncated ||
		content.File.ID != "binary" {
		t.Fatalf("bounded ReadDriveFile() = %#v", content)
	}

	exported, err := m.ReadDriveFile(context.Background(), "work", "doc", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Encoding != "base64" || exported.Content != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) ||
		exported.Truncated {
		t.Fatalf("exported ReadDriveFile() = %#v", exported)
	}

	if _, err := m.ReadDriveFile(context.Background(), "work", "folder", "", 10); err != ErrInvalidInput {
		t.Fatalf("folder ReadDriveFile() error = %v, want ErrInvalidInput", err)
	}
	if _, err := m.ReadDriveFile(context.Background(), "work", "", "", 10); err != ErrInvalidInput {
		t.Fatalf("blank ReadDriveFile() error = %v, want ErrInvalidInput", err)
	}
}

func newHTTPTestManager(t *testing.T, handler http.HandlerFunc) *Manager {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx := context.Background()
	options := []option.ClientOption{option.WithHTTPClient(server.Client())}
	gmail, err := gmailapi.NewService(ctx, options...)
	if err != nil {
		t.Fatal(err)
	}
	gmail.BasePath = server.URL + "/"
	calendar, err := calendarapi.NewService(ctx, options...)
	if err != nil {
		t.Fatal(err)
	}
	calendar.BasePath = server.URL + "/"
	drive, err := driveapi.NewService(ctx, options...)
	if err != nil {
		t.Fatal(err)
	}
	drive.BasePath = server.URL + "/"

	client := &Client{alias: "work", email: "reader@example.com", gmail: gmail, calendar: calendar, drive: drive}
	account := &managedAccount{alias: "work", email: "reader@example.com", client: client}
	return &Manager{
		cfg:      Config{DefaultAccount: "work"},
		accounts: map[string]*managedAccount{"work": account},
		emails:   map[string]*managedAccount{"reader@example.com": account},
	}
}

func assertQuery(t *testing.T, query url.Values, key, want string) {
	t.Helper()
	if got := query.Get(key); got != want {
		t.Errorf("query %q = %q, want %q", key, got, want)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}
