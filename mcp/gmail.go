package mcp

import (
	"context"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
)

type gmailSearchInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	Query   string `json:"query" jsonschema:"description=raw Gmail query syntax (e.g. in:inbox is:unread),required"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=results per page,minimum=1,maximum=100,default=20"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"description=opaque next_cursor from prior call"`
}

type gmailReadInput struct {
	Account   string `json:"account" jsonschema:"description=account alias or email,required"`
	MessageID string `json:"message_id" jsonschema:"description=Gmail message ID to fetch,required"`
}

type gmailThreadsInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	Query   string `json:"query,omitempty" jsonschema:"description=optional Gmail query to filter threads"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=threads per page,minimum=1,maximum=100,default=20"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"description=opaque next_cursor from prior call"`
}

type gmailSendInput struct {
	Account  string   `json:"account" jsonschema:"description=account alias or email,required"`
	To       []string `json:"to" jsonschema:"description=recipient emails,required"`
	Cc       []string `json:"cc,omitempty" jsonschema:"description=CC recipients"`
	Bcc      []string `json:"bcc,omitempty" jsonschema:"description=BCC recipients"`
	Subject  string   `json:"subject" jsonschema:"description=email subject,required"`
	Body     string   `json:"body,omitempty" jsonschema:"description=plain text body"`
	HTMLBody string   `json:"html_body,omitempty" jsonschema:"description=HTML body (used when body is empty)"`
}

type gmailReplyInput struct {
	Account  string `json:"account" jsonschema:"description=account alias or email,required"`
	ThreadID string `json:"thread_id" jsonschema:"description=thread ID to reply to,required"`
	Body     string `json:"body" jsonschema:"description=reply body content,required"`
}

type gmailForwardInput struct {
	Account   string   `json:"account" jsonschema:"description=account alias or email,required"`
	MessageID string   `json:"message_id" jsonschema:"description=message ID to forward,required"`
	To        []string `json:"to" jsonschema:"description=forward recipient emails,required"`
	Note      string   `json:"note,omitempty" jsonschema:"description=optional note above forwarded content"`
}

type gmailDraftInput struct {
	Account  string   `json:"account" jsonschema:"description=account alias or email,required"`
	To       []string `json:"to" jsonschema:"description=recipient emails,required"`
	Cc       []string `json:"cc,omitempty" jsonschema:"description=CC recipients"`
	Bcc      []string `json:"bcc,omitempty" jsonschema:"description=BCC recipients"`
	Subject  string   `json:"subject" jsonschema:"description=draft subject,required"`
	Body     string   `json:"body,omitempty" jsonschema:"description=plain text body"`
	HTMLBody string   `json:"html_body,omitempty" jsonschema:"description=HTML body (used when body is empty)"`
	ThreadID string   `json:"thread_id,omitempty" jsonschema:"description=thread ID to create the draft as a reply in (optional)"`
}

type accountOnlyInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
}

type gmailCreateLabelInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	Name    string `json:"name" jsonschema:"description=label name to create,required"`
}

type gmailDeleteLabelInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	LabelID string `json:"label_id" jsonschema:"description=label ID to delete,required"`
}

type gmailModifyLabelsInput struct {
	Account      string   `json:"account" jsonschema:"description=account alias or email,required"`
	MessageID    string   `json:"message_id" jsonschema:"description=message ID to modify,required"`
	AddLabels    []string `json:"add_labels,omitempty" jsonschema:"description=label IDs to add"`
	RemoveLabels []string `json:"remove_labels,omitempty" jsonschema:"description=label IDs to remove"`
}

type gmailMessageActionInput struct {
	Account   string `json:"account" jsonschema:"description=account alias or email,required"`
	MessageID string `json:"message_id" jsonschema:"description=message ID to mutate,required"`
}

func gmailSearch(ctx context.Context, m *google.Manager, in gmailSearchInput) (any, error) {
	res, err := m.SearchEmail(ctx, in.Account, in.Query, in.Limit, in.Cursor)
	if err != nil {
		return nil, err
	}
	return mcptool.PageOf(res.Items, res.NextCursor, in.Limit), nil
}

func gmailRead(ctx context.Context, m *google.Manager, in gmailReadInput) (any, error) {
	return m.ReadEmail(ctx, in.Account, in.MessageID)
}

func gmailListThreads(ctx context.Context, m *google.Manager, in gmailThreadsInput) (any, error) {
	res, err := m.ListThreads(ctx, in.Account, in.Query, in.Limit, in.Cursor)
	if err != nil {
		return nil, err
	}
	return mcptool.PageOf(res.Items, res.NextCursor, in.Limit), nil
}

func gmailSend(ctx context.Context, m *google.Manager, in gmailSendInput) (any, error) {
	return m.SendEmail(ctx, in.Account, google.SendEmailRequest{
		To:       in.To,
		Cc:       in.Cc,
		Bcc:      in.Bcc,
		Subject:  in.Subject,
		Body:     in.Body,
		HTMLBody: in.HTMLBody,
	})
}

func gmailReply(ctx context.Context, m *google.Manager, in gmailReplyInput) (any, error) {
	return m.ReplyEmail(ctx, in.Account, in.ThreadID, in.Body)
}

func gmailForward(ctx context.Context, m *google.Manager, in gmailForwardInput) (any, error) {
	return m.ForwardEmail(ctx, in.Account, in.MessageID, in.To, in.Note)
}

func gmailDraft(ctx context.Context, m *google.Manager, in gmailDraftInput) (any, error) {
	return m.CreateDraft(ctx, in.Account, in.ThreadID, google.SendEmailRequest{
		To:       in.To,
		Cc:       in.Cc,
		Bcc:      in.Bcc,
		Subject:  in.Subject,
		Body:     in.Body,
		HTMLBody: in.HTMLBody,
	})
}

func gmailListLabels(ctx context.Context, m *google.Manager, in accountOnlyInput) (any, error) {
	return m.ListLabels(ctx, in.Account)
}

func gmailCreateLabel(ctx context.Context, m *google.Manager, in gmailCreateLabelInput) (any, error) {
	return m.CreateLabel(ctx, in.Account, in.Name)
}

func gmailDeleteLabel(ctx context.Context, m *google.Manager, in gmailDeleteLabelInput) (any, error) {
	if err := m.DeleteLabel(ctx, in.Account, in.LabelID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "label_id": in.LabelID}, nil
}

func gmailModifyLabels(ctx context.Context, m *google.Manager, in gmailModifyLabelsInput) (any, error) {
	if err := m.ModifyLabels(ctx, in.Account, in.MessageID, in.AddLabels, in.RemoveLabels); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "message_id": in.MessageID}, nil
}

func gmailArchive(ctx context.Context, m *google.Manager, in gmailMessageActionInput) (any, error) {
	if err := m.ArchiveEmail(ctx, in.Account, in.MessageID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "message_id": in.MessageID}, nil
}

func gmailTrash(ctx context.Context, m *google.Manager, in gmailMessageActionInput) (any, error) {
	if err := m.TrashEmail(ctx, in.Account, in.MessageID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "message_id": in.MessageID}, nil
}

func gmailMarkRead(ctx context.Context, m *google.Manager, in gmailMessageActionInput) (any, error) {
	if err := m.MarkEmailRead(ctx, in.Account, in.MessageID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "message_id": in.MessageID}, nil
}

var gmailTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, gmailSearchInput](
		"google_gmail_search",
		"Search Gmail messages by query for one account",
		"SearchEmail",
		gmailSearch,
	),
	mcptool.Define[*google.Manager, gmailReadInput](
		"google_gmail_read",
		"Read one Gmail message with full body and headers",
		"ReadEmail",
		gmailRead,
	),
	mcptool.Define[*google.Manager, gmailThreadsInput](
		"google_gmail_list_threads",
		"List Gmail threads for one account with optional query filter",
		"ListThreads",
		gmailListThreads,
	),
	mcptool.Define[*google.Manager, gmailSendInput](
		"google_gmail_send",
		"Send a new Gmail message",
		"SendEmail",
		gmailSend,
	),
	mcptool.Define[*google.Manager, gmailReplyInput](
		"google_gmail_reply",
		"Reply to an existing Gmail thread",
		"ReplyEmail",
		gmailReply,
	),
	mcptool.Define[*google.Manager, gmailForwardInput](
		"google_gmail_forward",
		"Forward an existing Gmail message",
		"ForwardEmail",
		gmailForward,
	),
	mcptool.Define[*google.Manager, gmailDraftInput](
		"google_gmail_draft",
		"Create a Gmail draft message",
		"CreateDraft",
		gmailDraft,
	),
	mcptool.Define[*google.Manager, accountOnlyInput](
		"google_gmail_list_labels",
		"List Gmail labels for one account",
		"ListLabels",
		gmailListLabels,
	),
	mcptool.Define[*google.Manager, gmailCreateLabelInput](
		"google_gmail_create_label",
		"Create a Gmail label",
		"CreateLabel",
		gmailCreateLabel,
	),
	mcptool.Define[*google.Manager, gmailDeleteLabelInput](
		"google_gmail_delete_label",
		"Delete a Gmail label by ID",
		"DeleteLabel",
		gmailDeleteLabel,
	),
	mcptool.Define[*google.Manager, gmailModifyLabelsInput](
		"google_gmail_modify_labels",
		"Add or remove labels on a Gmail message",
		"ModifyLabels",
		gmailModifyLabels,
	),
	mcptool.Define[*google.Manager, gmailMessageActionInput](
		"google_gmail_archive",
		"Archive a Gmail message (remove INBOX label)",
		"ArchiveEmail",
		gmailArchive,
	),
	mcptool.Define[*google.Manager, gmailMessageActionInput](
		"google_gmail_trash",
		"Move a Gmail message to trash",
		"TrashEmail",
		gmailTrash,
	),
	mcptool.Define[*google.Manager, gmailMessageActionInput](
		"google_gmail_mark_read",
		"Mark a Gmail message as read",
		"MarkEmailRead",
		gmailMarkRead,
	),
}
