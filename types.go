package google

import (
	calendarapi "google.golang.org/api/calendar/v3"
	driveapi "google.golang.org/api/drive/v3"
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

// DriveFileSummary is a compact Google Drive file shape.
type DriveFileSummary struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MimeType     string   `json:"mime_type"`
	ModifiedTime string   `json:"modified_time,omitempty"`
	WebViewLink  string   `json:"web_view_link,omitempty"`
	Size         int64    `json:"size,omitempty"`
	Parents      []string `json:"parents,omitempty"`
}

// DriveFilesResult is a cursor-paginated Drive file page.
type DriveFilesResult struct {
	Items      []DriveFileSummary `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// DriveFileContent is the read response for one Drive file.
type DriveFileContent struct {
	File      DriveFileSummary `json:"file"`
	Content   string           `json:"content"`
	Encoding  string           `json:"encoding,omitempty"` // utf-8 or base64
	Truncated bool             `json:"truncated,omitempty"`
}

// DriveWriteRequest is used for file create operations.
type DriveWriteRequest struct {
	Name        string   `json:"name"`
	MimeType    string   `json:"mime_type,omitempty"`
	Content     string   `json:"content,omitempty"`
	Parents     []string `json:"parents,omitempty"`
	AsGoogleDoc bool     `json:"as_google_doc,omitempty"`
}

// DriveUpdateRequest is used for file update operations.
type DriveUpdateRequest struct {
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Content  string `json:"content,omitempty"`
}

// DriveComment is a compact comment on a Drive file, including Google Docs.
type DriveComment struct {
	ID           string `json:"id"`
	Content      string `json:"content,omitempty"`
	CreatedTime  string `json:"created_time,omitempty"`
	ModifiedTime string `json:"modified_time,omitempty"`
	Resolved     bool   `json:"resolved,omitempty"`
	Deleted      bool   `json:"deleted,omitempty"`
	AuthorName   string `json:"author_name,omitempty"`
}

// DriveCommentsResult is a cursor-paginated comment page.
type DriveCommentsResult struct {
	Items      []DriveComment `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// DocumentReplaceTextRequest defines a structured Google Docs text replacement.
type DocumentReplaceTextRequest struct {
	Find               string   `json:"find"`
	Replace            string   `json:"replace"`
	MatchCase          bool     `json:"match_case,omitempty"`
	TabIDs             []string `json:"tab_ids,omitempty"`
	RequiredRevisionID string   `json:"required_revision_id,omitempty"`
}

// DocumentMetadata contains the identifiers needed for conflict-safe editing.
type DocumentMetadata struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title,omitempty"`
	RevisionID string `json:"revision_id"`
}

// DocumentEditResult reports the result of a structured Google Docs edit.
type DocumentEditResult struct {
	DocumentID         string `json:"document_id"`
	RevisionID         string `json:"revision_id,omitempty"`
	OccurrencesChanged int64  `json:"occurrences_changed"`
}

// AccountDriveFile attaches a file to the account it came from.
type AccountDriveFile struct {
	Account string           `json:"account"`
	Email   string           `json:"email"`
	File    DriveFileSummary `json:"file"`
}

func toDriveComment(comment *driveapi.Comment) DriveComment {
	if comment == nil {
		return DriveComment{}
	}
	out := DriveComment{
		ID:           comment.Id,
		Content:      comment.Content,
		CreatedTime:  comment.CreatedTime,
		ModifiedTime: comment.ModifiedTime,
		Resolved:     comment.Resolved,
		Deleted:      comment.Deleted,
	}
	if comment.Author != nil {
		out.AuthorName = comment.Author.DisplayName
	}
	return out
}

func toDriveFileSummary(file *driveapi.File) DriveFileSummary {
	if file == nil {
		return DriveFileSummary{}
	}
	return DriveFileSummary{
		ID:           file.Id,
		Name:         file.Name,
		MimeType:     file.MimeType,
		ModifiedTime: file.ModifiedTime,
		WebViewLink:  file.WebViewLink,
		Size:         file.Size,
		Parents:      append([]string(nil), file.Parents...),
	}
}
