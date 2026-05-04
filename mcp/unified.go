package mcp

import (
	"context"
	"time"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
)

type unifiedInboxInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"description=max merged results,minimum=1,maximum=200,default=30"`
}

type unifiedSearchInput struct {
	Query string `json:"query" jsonschema:"description=Gmail query to run across all accounts,required"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=max merged results,minimum=1,maximum=200,default=30"`
}

type unifiedAgendaInput struct {
	TimeMin string `json:"time_min,omitempty" jsonschema:"description=RFC3339 lower time bound"`
	TimeMax string `json:"time_max,omitempty" jsonschema:"description=RFC3339 upper time bound"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=max merged events,minimum=1,maximum=1000,default=100"`
}

func unifiedInbox(ctx context.Context, m *google.Manager, in unifiedInboxInput) (any, error) {
	return m.UnifiedInbox(ctx, in.Limit)
}

func unifiedSearch(ctx context.Context, m *google.Manager, in unifiedSearchInput) (any, error) {
	return m.UnifiedSearch(ctx, in.Query, in.Limit)
}

func unifiedAgenda(ctx context.Context, m *google.Manager, in unifiedAgendaInput) (any, error) {
	min := time.Time{}
	max := time.Time{}
	if in.TimeMin != "" {
		parsed, err := time.Parse(time.RFC3339, in.TimeMin)
		if err != nil {
			return nil, err
		}
		min = parsed
	}
	if in.TimeMax != "" {
		parsed, err := time.Parse(time.RFC3339, in.TimeMax)
		if err != nil {
			return nil, err
		}
		max = parsed
	}
	return m.UnifiedAgenda(ctx, min, max, in.Limit)
}

var unifiedTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, unifiedInboxInput](
		"google_gmail_unified_inbox",
		"List unread inbox messages merged across all accounts",
		"UnifiedInbox",
		unifiedInbox,
	),
	mcptool.Define[*google.Manager, unifiedSearchInput](
		"google_gmail_unified_search",
		"Search Gmail across all configured accounts",
		"UnifiedSearch",
		unifiedSearch,
	),
	mcptool.Define[*google.Manager, unifiedAgendaInput](
		"google_calendar_unified_agenda",
		"List merged calendar events across all accounts",
		"UnifiedAgenda",
		unifiedAgenda,
	),
}
