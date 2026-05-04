package google

import (
	calendarapi "google.golang.org/api/calendar/v3"
)

// AccountInfo describes one configured account alias.
type AccountInfo struct {
	Alias         string `json:"alias"`
	Email         string `json:"email"`
	Authenticated bool   `json:"authenticated"`
}

// EmailSummary is the compact list shape returned by search/list operations.
type EmailSummary struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"thread_id"`
	Snippet      string   `json:"snippet,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	From         string   `json:"from,omitempty"`
	Date         string   `json:"date,omitempty"`
	InternalDate int64    `json:"internal_date,omitempty"`
	LabelIDs     []string `json:"label_ids,omitempty"`
}

// EmailMessage is the full message shape returned by read operations.
type EmailMessage struct {
	EmailSummary
	To   string `json:"to,omitempty"`
	Body string `json:"body,omitempty"`
}

// SearchEmailsResult is a cursor-paginated email page.
type SearchEmailsResult struct {
	Items      []EmailSummary `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ThreadSummary is a compact view of one Gmail thread.
type ThreadSummary struct {
	ID               string `json:"id"`
	MessageCount     int    `json:"message_count"`
	Snippet          string `json:"snippet,omitempty"`
	Subject          string `json:"subject,omitempty"`
	LastInternalDate int64  `json:"last_internal_date,omitempty"`
}

// ListThreadsResult is a cursor-paginated thread page.
type ListThreadsResult struct {
	Items      []ThreadSummary `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// SendEmailRequest defines fields for send/draft operations.
type SendEmailRequest struct {
	To       []string `json:"to"`
	Cc       []string `json:"cc,omitempty"`
	Bcc      []string `json:"bcc,omitempty"`
	Subject  string   `json:"subject"`
	Body     string   `json:"body,omitempty"`
	HTMLBody string   `json:"html_body,omitempty"`
}

// DraftResult is the compact result for draft creation.
type DraftResult struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id,omitempty"`
}

// LabelInfo is a compact Gmail label shape.
type LabelInfo struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type,omitempty"`
	MessageListVisibility string `json:"message_list_visibility,omitempty"`
	LabelListVisibility   string `json:"label_list_visibility,omitempty"`
}

// CalendarSummary is a compact calendar list shape.
type CalendarSummary struct {
	ID       string `json:"id"`
	Summary  string `json:"summary,omitempty"`
	TimeZone string `json:"time_zone,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

// CalendarEventsPage is a cursor-paginated calendar event page.
type CalendarEventsPage struct {
	Items      []*calendarapi.Event `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

// AccountEmail attaches a message to the account it came from.
type AccountEmail struct {
	Account string       `json:"account"`
	Email   string       `json:"email"`
	Message EmailSummary `json:"message"`
}

// AccountEvent attaches an event to the account+calendar it came from.
type AccountEvent struct {
	Account      string             `json:"account"`
	Email        string             `json:"email"`
	CalendarID   string             `json:"calendar_id"`
	CalendarName string             `json:"calendar_name,omitempty"`
	Event        *calendarapi.Event `json:"event"`
}
