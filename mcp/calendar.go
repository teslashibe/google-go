package mcp

import (
	"context"
	"time"

	google "github.com/teslashibe/google-go"
	"github.com/teslashibe/mcptool"
	calendarapi "google.golang.org/api/calendar/v3"
)

type calendarListInput struct {
	Account string `json:"account" jsonschema:"description=account alias or email,required"`
}

type calendarEventsInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	TimeMin    string `json:"time_min,omitempty" jsonschema:"description=RFC3339 lower time bound"`
	TimeMax    string `json:"time_max,omitempty" jsonschema:"description=RFC3339 upper time bound"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=events per page,minimum=1,maximum=250,default=25"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"description=opaque next_cursor from prior call"`
}

type calendarGetEventInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	EventID    string `json:"event_id" jsonschema:"description=Google Calendar event ID,required"`
}

type calendarFreeBusyInput struct {
	Account     string   `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarIDs []string `json:"calendar_ids,omitempty" jsonschema:"description=calendar IDs to evaluate (defaults to primary)"`
	TimeMin     string   `json:"time_min,omitempty" jsonschema:"description=RFC3339 lower time bound"`
	TimeMax     string   `json:"time_max,omitempty" jsonschema:"description=RFC3339 upper time bound"`
}

type calendarCreateEventInput struct {
	Account    string            `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string            `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	Event      calendarapi.Event `json:"event" jsonschema:"description=Calendar event payload,required"`
}

type calendarUpdateEventInput struct {
	Account    string            `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string            `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	EventID    string            `json:"event_id" jsonschema:"description=event ID to update,required"`
	Event      calendarapi.Event `json:"event" jsonschema:"description=partial event payload to patch,required"`
}

type calendarDeleteEventInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	EventID    string `json:"event_id" jsonschema:"description=event ID to delete,required"`
}

type calendarRSVPInput struct {
	Account    string `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	EventID    string `json:"event_id" jsonschema:"description=event ID to RSVP to,required"`
	Response   string `json:"response" jsonschema:"description=accepted|declined|tentative,required"`
}

func calendarList(ctx context.Context, m *google.Manager, in calendarListInput) (any, error) {
	return m.ListCalendars(ctx, in.Account)
}

func calendarEvents(ctx context.Context, m *google.Manager, in calendarEventsInput) (any, error) {
	timeMin, err := parseOptionalRFC3339(in.TimeMin)
	if err != nil {
		return nil, err
	}
	timeMax, err := parseOptionalRFC3339(in.TimeMax)
	if err != nil {
		return nil, err
	}
	res, err := m.ListEvents(ctx, in.Account, in.CalendarID, timeMin, timeMax, in.Limit, in.Cursor)
	if err != nil {
		return nil, err
	}
	return mcptool.PageOf(res.Items, res.NextCursor, in.Limit), nil
}

func calendarGetEvent(ctx context.Context, m *google.Manager, in calendarGetEventInput) (any, error) {
	return m.GetEvent(ctx, in.Account, in.CalendarID, in.EventID)
}

func calendarFreeBusy(ctx context.Context, m *google.Manager, in calendarFreeBusyInput) (any, error) {
	timeMin, err := parseOptionalRFC3339(in.TimeMin)
	if err != nil {
		return nil, err
	}
	timeMax, err := parseOptionalRFC3339(in.TimeMax)
	if err != nil {
		return nil, err
	}
	min := time.Now()
	max := min.Add(24 * time.Hour)
	if timeMin != nil {
		min = *timeMin
	}
	if timeMax != nil {
		max = *timeMax
	}
	return m.FreeBusy(ctx, in.Account, in.CalendarIDs, min, max)
}

func calendarCreateEvent(ctx context.Context, m *google.Manager, in calendarCreateEventInput) (any, error) {
	event := in.Event
	return m.CreateEvent(ctx, in.Account, in.CalendarID, &event)
}

func calendarUpdateEvent(ctx context.Context, m *google.Manager, in calendarUpdateEventInput) (any, error) {
	event := in.Event
	return m.UpdateEvent(ctx, in.Account, in.CalendarID, in.EventID, &event)
}

func calendarDeleteEvent(ctx context.Context, m *google.Manager, in calendarDeleteEventInput) (any, error) {
	if err := m.DeleteEvent(ctx, in.Account, in.CalendarID, in.EventID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "event_id": in.EventID}, nil
}

func calendarRSVP(ctx context.Context, m *google.Manager, in calendarRSVPInput) (any, error) {
	return m.RSVPEvent(ctx, in.Account, in.CalendarID, in.EventID, in.Response)
}

var calendarTools = []mcptool.Tool{
	mcptool.Define[*google.Manager, calendarListInput](
		"google_calendar_list",
		"List available calendars for one account",
		"ListCalendars",
		calendarList,
	),
	mcptool.Define[*google.Manager, calendarEventsInput](
		"google_calendar_events",
		"List calendar events in a time window",
		"ListEvents",
		calendarEvents,
	),
	mcptool.Define[*google.Manager, calendarGetEventInput](
		"google_calendar_get_event",
		"Fetch one calendar event by ID",
		"GetEvent",
		calendarGetEvent,
	),
	mcptool.Define[*google.Manager, calendarFreeBusyInput](
		"google_calendar_free_busy",
		"Query free/busy windows for one or more calendars",
		"FreeBusy",
		calendarFreeBusy,
	),
	mcptool.Define[*google.Manager, calendarCreateEventInput](
		"google_calendar_create_event",
		"Create a new Google Calendar event",
		"CreateEvent",
		calendarCreateEvent,
	),
	mcptool.Define[*google.Manager, calendarUpdateEventInput](
		"google_calendar_update_event",
		"Patch an existing Google Calendar event",
		"UpdateEvent",
		calendarUpdateEvent,
	),
	mcptool.Define[*google.Manager, calendarDeleteEventInput](
		"google_calendar_delete_event",
		"Delete a Google Calendar event",
		"DeleteEvent",
		calendarDeleteEvent,
	),
	mcptool.Define[*google.Manager, calendarRSVPInput](
		"google_calendar_rsvp",
		"RSVP accepted/declined/tentative on an event",
		"RSVPEvent",
		calendarRSVP,
	),
}

func parseOptionalRFC3339(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
