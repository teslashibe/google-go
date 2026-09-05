package mcp

import (
	"context"
	"fmt"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
)

type docsReplaceTextInput struct {
	Account            string   `json:"account" jsonschema:"description=account alias or email,required"`
	DocumentID         string   `json:"document_id" jsonschema:"description=Google Docs document ID,required"`
	Find               string   `json:"find" jsonschema:"description=literal text to replace,required"`
	Replace            string   `json:"replace" jsonschema:"description=replacement text,required"`
	MatchCase          bool     `json:"match_case,omitempty" jsonschema:"description=match case exactly"`
	TabIDs             []string `json:"tab_ids,omitempty" jsonschema:"description=optional Google Docs tab IDs; omit to replace across all tabs"`
	RequiredRevisionID string   `json:"required_revision_id,omitempty" jsonschema:"description=optional revision precondition to prevent overwriting concurrent edits"`
}

type docsGetMetadataInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	DocumentID string `json:"document_id" jsonschema:"description=Google Docs document ID,required"`
}

type docsProposeChangeCommentInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	DocumentID string `json:"document_id" jsonschema:"description=Google Docs document ID,required"`
	Find       string `json:"find" jsonschema:"description=text whose replacement is proposed,required"`
	Replace    string `json:"replace" jsonschema:"description=proposed replacement text,required"`
	Note       string `json:"note,omitempty" jsonschema:"description=optional rationale for the proposed change"`
}

func docsReplaceText(ctx context.Context, m *google.Manager, in docsReplaceTextInput) (any, error) {
	return m.ReplaceDocumentText(ctx, in.Account, in.DocumentID, google.DocumentReplaceTextRequest{
		Find:               in.Find,
		Replace:            in.Replace,
		MatchCase:          in.MatchCase,
		TabIDs:             in.TabIDs,
		RequiredRevisionID: in.RequiredRevisionID,
	})
}

func docsGetMetadata(ctx context.Context, m *google.Manager, in docsGetMetadataInput) (any, error) {
	return m.GetDocumentMetadata(ctx, in.Account, in.DocumentID)
}

func docsProposeChangeComment(ctx context.Context, m *google.Manager, in docsProposeChangeCommentInput) (any, error) {
	content := fmt.Sprintf("Proposed change:\nReplace: %q\nWith: %q", in.Find, in.Replace)
	if in.Note != "" {
		content += "\n\nReason: " + in.Note
	}
	return m.CreateDriveComment(ctx, in.Account, in.DocumentID, content)
}

var docsTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, docsGetMetadataInput](
		"google_docs_get_metadata",
		"Get a Google Doc title and current revision ID for conflict-safe edits",
		"GetDocumentMetadata",
		docsGetMetadata,
	),
	mcptool.Define[*google.Manager, docsReplaceTextInput](
		"google_docs_replace_text",
		"Directly replace literal text in a Google Doc; use required_revision_id for conflict-safe edits",
		"ReplaceDocumentText",
		docsReplaceText,
	),
	mcptool.Define[*google.Manager, docsProposeChangeCommentInput](
		"google_docs_propose_change_comment",
		"Add a document-level comment proposing a text replacement; this is not a native Google Docs suggestion",
		"CreateDriveComment",
		docsProposeChangeComment,
	),
}
