package mcp

import (
	"context"
	"fmt"
	"slices"
	"strings"
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
	// PrivateProperty filters as Google's privateExtendedProperty: each is
	// key=value and an event must carry every one.
	PrivateProperty []string `json:"private_property,omitempty" jsonschema:"description=private extended property filters as key=value; events must match all"`
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
	Account           string            `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID        string            `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	Event             calendarapi.Event `json:"event" jsonschema:"description=Calendar event payload,required"`
	Transparency      string            `json:"transparency,omitempty" jsonschema:"description=opaque (busy) or transparent (free),enum=opaque,enum=transparent"`
	Visibility        string            `json:"visibility,omitempty" jsonschema:"description=default or public or private,enum=default,enum=public,enum=private"`
	PrivateProperties map[string]string `json:"private_properties,omitempty" jsonschema:"description=private extended properties to set (string map)"`
}

type calendarUpdateEventInput struct {
	Account           string            `json:"account" jsonschema:"description=account alias or email,required"`
	CalendarID        string            `json:"calendar_id,omitempty" jsonschema:"description=calendar ID (defaults to primary)"`
	EventID           string            `json:"event_id" jsonschema:"description=event ID to update,required"`
	Event             calendarapi.Event `json:"event" jsonschema:"description=partial event payload to patch,required"`
	Transparency      string            `json:"transparency,omitempty" jsonschema:"description=opaque (busy) or transparent (free),enum=opaque,enum=transparent"`
	Visibility        string            `json:"visibility,omitempty" jsonschema:"description=default or public or private,enum=default,enum=public,enum=private"`
	PrivateProperties map[string]string `json:"private_properties,omitempty" jsonschema:"description=private extended properties to set (string map)"`
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
	opts := make([]google.ListEventsOption, 0, len(in.PrivateProperty))
	for _, filter := range in.PrivateProperty {
		key, value, ok := strings.Cut(filter, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, invalidInput("private_property %q is not key=value", filter)
		}
		opts = append(opts, google.WithPrivateProperty(key, value))
	}
	res, err := m.ListEvents(ctx, in.Account, in.CalendarID, timeMin, timeMax, in.Limit, in.Cursor, opts...)
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
	if err := applyEventFields(&event, in.Transparency, in.Visibility, in.PrivateProperties); err != nil {
		return nil, err
	}
	return m.CreateEvent(ctx, in.Account, in.CalendarID, &event)
}

func calendarUpdateEvent(ctx context.Context, m *google.Manager, in calendarUpdateEventInput) (any, error) {
	event := in.Event
	if err := applyEventFields(&event, in.Transparency, in.Visibility, in.PrivateProperties); err != nil {
		return nil, err
	}
	return m.UpdateEvent(ctx, in.Account, in.CalendarID, in.EventID, &event)
}

// applyEventFields sets the top-level transparency, visibility and private
// properties on the event payload. They win over the same fields in event.
func applyEventFields(event *calendarapi.Event, transparency, visibility string, private map[string]string) error {
	switch transparency {
	case "", "opaque", "transparent":
	default:
		return invalidInput("transparency %q must be opaque or transparent", transparency)
	}
	switch visibility {
	case "", "default", "public", "private":
	default:
		return invalidInput("visibility %q must be default, public or private", visibility)
	}
	// Go drops a false reminders.useDefault from the JSON, so a payload that
	// turns the calendar's default reminders off would get them: send it.
	if event.Reminders != nil && !event.Reminders.UseDefault && !slices.Contains(event.Reminders.ForceSendFields, "UseDefault") {
		event.Reminders.ForceSendFields = append(event.Reminders.ForceSendFields, "UseDefault")
	}
	if transparency != "" {
		event.Transparency = transparency
	}
	if visibility != "" {
		event.Visibility = visibility
	}
	if len(private) > 0 {
		props := &calendarapi.EventExtendedProperties{}
		if event.ExtendedProperties != nil {
			copied := *event.ExtendedProperties
			props = &copied
		}
		merged := make(map[string]string, len(props.Private)+len(private))
		for k, v := range props.Private {
			merged[k] = v
		}
		for k, v := range private {
			if strings.TrimSpace(k) == "" {
				return invalidInput("private_properties has an empty key")
			}
			merged[k] = v
		}
		props.Private = merged
		event.ExtendedProperties = props
	}
	return nil
}

func invalidInput(format string, args ...any) error {
	return &mcptool.Error{Code: "invalid_input", Message: fmt.Sprintf(format, args...)}
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
		"List calendar events in a time window, optionally by private property",
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
		"Create a Google Calendar event with optional transparency, visibility and private properties",
		"CreateEvent",
		calendarCreateEvent,
	),
	mcptool.Define[*google.Manager, calendarUpdateEventInput](
		"google_calendar_update_event",
		"Patch a Google Calendar event, including transparency, visibility and private properties",
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
