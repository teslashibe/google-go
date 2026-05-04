package mcp

import (
	"context"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
)

type driveListInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	Query   string `json:"query,omitempty" jsonschema:"description=Drive query syntax (defaults to non-trashed files)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=files per page,minimum=1,maximum=200,default=30"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"description=opaque next_cursor from prior call"`
}

type driveReadInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	FileID     string `json:"file_id" jsonschema:"description=Drive file ID to read,required"`
	ExportMIME string `json:"export_mime,omitempty" jsonschema:"description=export MIME for Google-native files (e.g. text/plain,text/csv)"`
	MaxBytes   int    `json:"max_bytes,omitempty" jsonschema:"description=max bytes to return,minimum=1,maximum=1048576,default=524288"`
}

type driveCreateInput struct {
	Account     string   `json:"account" jsonschema:"description=account alias or email,required"`
	Name        string   `json:"name" jsonschema:"description=file name to create,required"`
	MimeType    string   `json:"mime_type,omitempty" jsonschema:"description=optional file MIME type"`
	Content     string   `json:"content,omitempty" jsonschema:"description=file text content"`
	Parents     []string `json:"parents,omitempty" jsonschema:"description=optional parent folder IDs"`
	AsGoogleDoc bool     `json:"as_google_doc,omitempty" jsonschema:"description=create as Google Doc type (conversion)"`
}

type driveUpdateInput struct {
	Account  string `json:"account" jsonschema:"description=account alias or email,required"`
	FileID   string `json:"file_id" jsonschema:"description=Drive file ID to update,required"`
	Name     string `json:"name,omitempty" jsonschema:"description=new file name"`
	MimeType string `json:"mime_type,omitempty" jsonschema:"description=new MIME type"`
	Content  string `json:"content,omitempty" jsonschema:"description=replacement file content"`
}

type driveDeleteInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
	FileID  string `json:"file_id" jsonschema:"description=Drive file ID to delete,required"`
}

type driveUnifiedSearchInput struct {
	Query string `json:"query,omitempty" jsonschema:"description=Drive query syntax across all accounts"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=max merged results,minimum=1,maximum=300,default=30"`
}

func driveList(ctx context.Context, m *google.Manager, in driveListInput) (any, error) {
	res, err := m.ListDriveFiles(ctx, in.Account, in.Query, in.Limit, in.Cursor)
	if err != nil {
		return nil, err
	}
	return mcptool.PageOf(res.Items, res.NextCursor, in.Limit), nil
}

func driveRead(ctx context.Context, m *google.Manager, in driveReadInput) (any, error) {
	return m.ReadDriveFile(ctx, in.Account, in.FileID, in.ExportMIME, in.MaxBytes)
}

func driveCreate(ctx context.Context, m *google.Manager, in driveCreateInput) (any, error) {
	return m.CreateDriveFile(ctx, in.Account, google.DriveWriteRequest{
		Name:        in.Name,
		MimeType:    in.MimeType,
		Content:     in.Content,
		Parents:     in.Parents,
		AsGoogleDoc: in.AsGoogleDoc,
	})
}

func driveUpdate(ctx context.Context, m *google.Manager, in driveUpdateInput) (any, error) {
	return m.UpdateDriveFile(ctx, in.Account, in.FileID, google.DriveUpdateRequest{
		Name:     in.Name,
		MimeType: in.MimeType,
		Content:  in.Content,
	})
}

func driveDelete(ctx context.Context, m *google.Manager, in driveDeleteInput) (any, error) {
	if err := m.DeleteDriveFile(ctx, in.Account, in.FileID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "file_id": in.FileID}, nil
}

func driveUnifiedSearch(ctx context.Context, m *google.Manager, in driveUnifiedSearchInput) (any, error) {
	return m.UnifiedDriveSearch(ctx, in.Query, in.Limit)
}

var driveTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, driveListInput](
		"google_drive_list",
		"List Drive files for one account",
		"ListDriveFiles",
		driveList,
	),
	mcptool.Define[*google.Manager, driveReadInput](
		"google_drive_read",
		"Read one Drive file (or export Google-native content)",
		"ReadDriveFile",
		driveRead,
	),
	mcptool.Define[*google.Manager, driveCreateInput](
		"google_drive_create",
		"Create a Drive file with optional content",
		"CreateDriveFile",
		driveCreate,
	),
	mcptool.Define[*google.Manager, driveUpdateInput](
		"google_drive_update",
		"Update Drive file metadata or content",
		"UpdateDriveFile",
		driveUpdate,
	),
	mcptool.Define[*google.Manager, driveDeleteInput](
		"google_drive_delete",
		"Delete a Drive file by ID",
		"DeleteDriveFile",
		driveDelete,
	),
	mcptool.Define[*google.Manager, driveUnifiedSearchInput](
		"google_drive_unified_search",
		"Search Drive files across all authenticated accounts",
		"UnifiedDriveSearch",
		driveUnifiedSearch,
	),
}
