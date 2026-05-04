package mcp

import (
	"context"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
)

type authenticateInput struct {
	Account string `json:"account" jsonschema:"description=account alias to configure (e.g. work),required"`
	Email   string `json:"email" jsonschema:"description=google account email for this alias,required"`
}

type listAccountsInput struct {
	IncludeUnauthenticated bool `json:"include_unauthenticated,omitempty" jsonschema:"description=include aliases that exist but are not currently authenticated,default=true"`
}

func listAccounts(_ context.Context, m *google.Manager, in listAccountsInput) (any, error) {
	all := m.ListAccounts()
	if in.IncludeUnauthenticated {
		return all, nil
	}
	out := make([]google.AccountInfo, 0, len(all))
	for _, a := range all {
		if a.Authenticated {
			out = append(out, a)
		}
	}
	return out, nil
}

func authenticate(ctx context.Context, m *google.Manager, in authenticateInput) (any, error) {
	return m.Authenticate(ctx, in.Account, in.Email)
}

var accountTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, listAccountsInput](
		"google_gmail_list_accounts",
		"List configured Google account aliases and auth status",
		"ListAccounts",
		listAccounts,
	),
	mcptool.Define[*google.Manager, authenticateInput](
		"google_gmail_authenticate",
		"Authenticate a Google account alias via OAuth browser flow",
		"Authenticate",
		authenticate,
	),
}
