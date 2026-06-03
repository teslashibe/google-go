package google

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/oauth2"
	calendarapi "google.golang.org/api/calendar/v3"
	driveapi "google.golang.org/api/drive/v3"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

var defaultGoogleScopes = []string{
	gmailapi.GmailModifyScope,
	gmailapi.GmailSendScope,
	calendarapi.CalendarEventsScope,
	calendarapi.CalendarReadonlyScope,
	driveapi.DriveScope,
}

const maxDriveReadBytes = 512 * 1024

// Client wraps one authenticated Google account.
type Client struct {
	alias       string
	email       string
	gmail       *gmailapi.Service
	calendar    *calendarapi.Service
	drive       *driveapi.Service
	tokenSource oauth2.TokenSource
}

type managedAccount struct {
	alias     string
	email     string
	tokenPath string
	client    *Client
}

// Manager handles N Google accounts under one config directory.
type Manager struct {
	mu            sync.RWMutex
	configPath    string
	configDir     string
	oauthKeysPath string

	store       *fileConfigStore
	cfg         Config
	oauthConfig *oauth2.Config

	accounts map[string]*managedAccount // alias (lowercase) => account
	emails   map[string]*managedAccount // email (lowercase) => account
}

// NewManagerFromInlineToken creates a single-account Manager from an OAuth2
// token and the shared oauth-keys.json, without any filesystem config
// directory. Used by the web OAuth callback flow where the token is stored
// encrypted in the DB rather than on disk.
func NewManagerFromInlineToken(oauthKeysPath string, token *oauth2.Token, alias, email string) (*Manager, error) {
	oauthCfg, err := loadOAuth2Config(oauthKeysPath, defaultGoogleScopes)
	if err != nil {
		return nil, err
	}
	client, err := newClientStateless(context.Background(), oauthCfg, alias, email, token)
	if err != nil {
		return nil, err
	}
	entry := &managedAccount{alias: alias, email: email, client: client}
	m := &Manager{
		oauthConfig: oauthCfg,
		cfg:         Config{DefaultAccount: alias, Accounts: map[string]AccountConfig{alias: {Email: email}}},
		accounts:    map[string]*managedAccount{alias: entry},
		emails:      map[string]*managedAccount{strings.ToLower(email): entry},
	}
	return m, nil
}

// newClientStateless is like newClient but does not persist token refreshes
// to disk. Token persistence is handled by the credentials.Service layer.
func newClientStateless(ctx context.Context, oauthCfg *oauth2.Config, alias, email string, token *oauth2.Token) (*Client, error) {
	if oauthCfg == nil {
		return nil, ErrOAuthKeysNotFound
	}
	if token == nil {
		return nil, ErrAccountNotAuthenticated
	}
	ts := oauthCfg.TokenSource(ctx, token)
	httpClient := oauth2.NewClient(ctx, ts)
	gmailSvc, err := gmailapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new gmail service: %w", err)
	}
	calSvc, err := calendarapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new calendar service: %w", err)
	}
	driveSvc, err := driveapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new drive service: %w", err)
	}
	return &Client{
		alias:       alias,
		email:       email,
		gmail:       gmailSvc,
		calendar:    calSvc,
		drive:       driveSvc,
		tokenSource: ts,
	}, nil
}

// LoadOAuth2Config is the exported version of loadOAuth2Config for use by
// external packages (e.g. the web OAuth callback handler).
func LoadOAuth2Config(keysPath string, scopes []string) (*oauth2.Config, error) {
	return loadOAuth2Config(keysPath, scopes)
}

// DefaultGoogleScopes returns the default scopes used by the Manager.
func DefaultGoogleScopes() []string {
	out := make([]string, len(defaultGoogleScopes))
	copy(out, defaultGoogleScopes)
	return out
}

// NewManager loads config and any already-authenticated accounts.
func NewManager(configPath string) (*Manager, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = DefaultConfigPath()
	}
	configPath = filepath.Clean(configPath)

	store := newFileConfigStore(configPath)
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}

	m := &Manager{
		configPath:    configPath,
		configDir:     configDirFromPath(configPath),
		oauthKeysPath: filepath.Join(configDirFromPath(configPath), oauthKeysFileName),
		store:         store,
		cfg:           cfg,
		accounts:      map[string]*managedAccount{},
		emails:        map[string]*managedAccount{},
	}
	if err := m.bootstrap(context.Background()); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) bootstrap(ctx context.Context) error {
	if cfg, err := loadOAuth2Config(m.oauthKeysPath, defaultGoogleScopes); err == nil {
		m.oauthConfig = cfg
	} else if !errors.Is(err, ErrOAuthKeysNotFound) {
		return err
	}

	for rawAlias, ac := range m.cfg.Accounts {
		alias := normalizeAlias(rawAlias)
		if alias == "" {
			continue
		}
		entry := &managedAccount{
			alias:     alias,
			email:     strings.TrimSpace(ac.Email),
			tokenPath: resolveTokenPath(m.configDir, alias, ac.TokenPath),
		}
		m.accounts[alias] = entry
		if entry.email != "" {
			m.emails[strings.ToLower(entry.email)] = entry
		}
		if m.oauthConfig == nil {
			continue
		}
		tok, err := loadToken(entry.tokenPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		client, err := newClient(ctx, m.oauthConfig, entry.alias, entry.email, entry.tokenPath, tok)
		if err != nil {
			continue
		}
		entry.client = client
	}

	return nil
}

// ListAccounts returns every configured alias and auth status.
func (m *Manager) ListAccounts() []AccountInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]AccountInfo, 0, len(m.accounts))
	for _, ac := range m.accounts {
		out = append(out, AccountInfo{
			Alias:         ac.alias,
			Email:         ac.email,
			Authenticated: ac.client != nil,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}

// Resolve returns a live account client from alias/email/default.
func (m *Manager) Resolve(account string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ac, err := m.resolveAccountLocked(account)
	if err != nil {
		return nil, err
	}
	if ac.client == nil {
		return nil, ErrAccountNotAuthenticated
	}
	return ac.client, nil
}

// Authenticate starts OAuth for alias/email, stores token, and activates account.
func (m *Manager) Authenticate(ctx context.Context, alias, email string) (AccountInfo, error) {
	alias = normalizeAlias(alias)
	email = strings.TrimSpace(email)
	if alias == "" || email == "" {
		return AccountInfo{}, ErrInvalidInput
	}

	oauthCfg, err := m.ensureOAuthConfig()
	if err != nil {
		return AccountInfo{}, err
	}
	tok, err := runOAuthBrowserFlow(ctx, oauthCfg)
	if err != nil {
		return AccountInfo{}, err
	}

	m.mu.Lock()
	existingCfg := m.cfg.Accounts[alias]
	tokenPath := resolveTokenPath(m.configDir, alias, existingCfg.TokenPath)
	m.mu.Unlock()

	if err := saveToken(tokenPath, tok); err != nil {
		return AccountInfo{}, err
	}
	client, err := newClient(ctx, oauthCfg, alias, email, tokenPath, tok)
	if err != nil {
		return AccountInfo{}, err
	}

	relToken := relativeTokenPath(m.configDir, tokenPath)

	m.mu.Lock()
	defer m.mu.Unlock()

	nextCfg := m.cfg
	if nextCfg.Accounts == nil {
		nextCfg.Accounts = map[string]AccountConfig{}
	}
	accountsCopy := make(map[string]AccountConfig, len(nextCfg.Accounts)+1)
	for k, v := range nextCfg.Accounts {
		accountsCopy[k] = v
	}
	nextCfg.Accounts = accountsCopy
	nextCfg.Accounts[alias] = AccountConfig{
		Email:     email,
		TokenPath: relToken,
	}
	if nextCfg.DefaultAccount == "" {
		nextCfg.DefaultAccount = alias
	}

	if err := m.store.Save(nextCfg); err != nil {
		return AccountInfo{}, err
	}
	m.cfg = nextCfg

	oldEmail := ""
	if old := m.accounts[alias]; old != nil {
		oldEmail = old.email
	}
	if oldEmail != "" && strings.ToLower(oldEmail) != strings.ToLower(email) {
		delete(m.emails, strings.ToLower(oldEmail))
	}

	entry := &managedAccount{
		alias:     alias,
		email:     email,
		tokenPath: tokenPath,
		client:    client,
	}
	m.accounts[alias] = entry
	m.emails[strings.ToLower(email)] = entry

	return AccountInfo{
		Alias:         alias,
		Email:         email,
		Authenticated: true,
	}, nil
}

// SearchEmail searches one account inbox/query and returns compact summaries.
func (m *Manager) SearchEmail(ctx context.Context, account, query string, limit int, cursor string) (SearchEmailsResult, error) {
	if strings.TrimSpace(query) == "" {
		return SearchEmailsResult{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return SearchEmailsResult{}, err
	}
	limit = normalizeLimit(limit, 20, 100)
	call := client.gmail.Users.Messages.List("me").Q(query).MaxResults(int64(limit))
	if cursor != "" {
		call = call.PageToken(cursor)
	}
	res, err := call.Context(ctx).Do()
	if err != nil {
		return SearchEmailsResult{}, err
	}

	out := SearchEmailsResult{Items: make([]EmailSummary, 0, len(res.Messages)), NextCursor: res.NextPageToken}
	for _, item := range res.Messages {
		msg, err := client.gmail.Users.Messages.Get("me", item.Id).
			Format("metadata").
			MetadataHeaders("Subject", "From", "Date").
			Context(ctx).
			Do()
		if err != nil {
			continue
		}
		out.Items = append(out.Items, toEmailSummary(msg))
	}
	return out, nil
}

// ReadEmail fetches one full email (body + key headers).
func (m *Manager) ReadEmail(ctx context.Context, account, messageID string) (EmailMessage, error) {
	if strings.TrimSpace(messageID) == "" {
		return EmailMessage{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return EmailMessage{}, err
	}
	msg, err := client.gmail.Users.Messages.Get("me", messageID).Format("full").Context(ctx).Do()
	if err != nil {
		return EmailMessage{}, err
	}

	headers := messageHeaders(msg)
	s := toEmailSummary(msg)
	return EmailMessage{
		EmailSummary: s,
		To:           firstNonEmpty(headers["To"], headers["Delivered-To"]),
		Body:         messageBody(msg),
	}, nil
}

// ListThreads searches/list threads on one account.
func (m *Manager) ListThreads(ctx context.Context, account, query string, limit int, cursor string) (ListThreadsResult, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return ListThreadsResult{}, err
	}
	limit = normalizeLimit(limit, 20, 100)
	call := client.gmail.Users.Threads.List("me").MaxResults(int64(limit))
	if query != "" {
		call = call.Q(query)
	}
	if cursor != "" {
		call = call.PageToken(cursor)
	}
	res, err := call.Context(ctx).Do()
	if err != nil {
		return ListThreadsResult{}, err
	}

	out := ListThreadsResult{Items: make([]ThreadSummary, 0, len(res.Threads)), NextCursor: res.NextPageToken}
	for _, t := range res.Threads {
		full, err := client.gmail.Users.Threads.Get("me", t.Id).
			Format("metadata").
			MetadataHeaders("Subject", "Date").
			Context(ctx).
			Do()
		if err != nil {
			continue
		}
		summary := ThreadSummary{
			ID:           full.Id,
			MessageCount: len(full.Messages),
		}
		if n := len(full.Messages); n > 0 {
			last := full.Messages[n-1]
			summary.Snippet = last.Snippet
			summary.LastInternalDate = last.InternalDate
			summary.Subject = messageHeaders(last)["Subject"]
		}
		out.Items = append(out.Items, summary)
	}
	return out, nil
}

// SendEmail sends a new email from one account.
func (m *Manager) SendEmail(ctx context.Context, account string, req SendEmailRequest) (EmailSummary, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return EmailSummary{}, err
	}
	raw, err := buildRawMessage(req, nil)
	if err != nil {
		return EmailSummary{}, err
	}
	sent, err := client.gmail.Users.Messages.Send("me", &gmailapi.Message{Raw: raw}).Context(ctx).Do()
	if err != nil {
		return EmailSummary{}, err
	}
	return m.fetchSummary(ctx, client, sent.Id)
}

// ReplyEmail replies to an existing thread.
func (m *Manager) ReplyEmail(ctx context.Context, account, threadID, body string) (EmailSummary, error) {
	if strings.TrimSpace(threadID) == "" || strings.TrimSpace(body) == "" {
		return EmailSummary{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return EmailSummary{}, err
	}
	thread, err := client.gmail.Users.Threads.Get("me", threadID).
		Format("metadata").
		MetadataHeaders("Subject", "From", "Reply-To", "Message-ID", "References").
		Context(ctx).
		Do()
	if err != nil {
		return EmailSummary{}, err
	}
	if len(thread.Messages) == 0 {
		return EmailSummary{}, ErrInvalidInput
	}
	last := thread.Messages[len(thread.Messages)-1]
	h := messageHeaders(last)
	to := firstEmailAddress(firstNonEmpty(h["Reply-To"], h["From"]))
	if to == "" {
		return EmailSummary{}, ErrInvalidInput
	}
	subject := ensureSubjectPrefix(h["Subject"], "Re:")
	references := strings.TrimSpace(strings.TrimSpace(h["References"]) + " " + strings.TrimSpace(h["Message-ID"]))
	raw, err := buildRawMessage(SendEmailRequest{
		To:      []string{to},
		Subject: subject,
		Body:    body,
	}, map[string]string{
		"In-Reply-To": h["Message-ID"],
		"References":  references,
	})
	if err != nil {
		return EmailSummary{}, err
	}
	sent, err := client.gmail.Users.Messages.Send("me", &gmailapi.Message{
		ThreadId: threadID,
		Raw:      raw,
	}).Context(ctx).Do()
	if err != nil {
		return EmailSummary{}, err
	}
	return m.fetchSummary(ctx, client, sent.Id)
}

// ForwardEmail forwards one message to new recipients.
func (m *Manager) ForwardEmail(ctx context.Context, account, messageID string, to []string, note string) (EmailSummary, error) {
	if strings.TrimSpace(messageID) == "" || len(to) == 0 {
		return EmailSummary{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return EmailSummary{}, err
	}
	source, err := client.gmail.Users.Messages.Get("me", messageID).Format("full").Context(ctx).Do()
	if err != nil {
		return EmailSummary{}, err
	}
	h := messageHeaders(source)
	forwarded := "---- Forwarded message ----\n" +
		"From: " + firstNonEmpty(h["From"], "(unknown)") + "\n" +
		"Date: " + firstNonEmpty(h["Date"], "(unknown)") + "\n" +
		"Subject: " + firstNonEmpty(h["Subject"], "(no subject)") + "\n\n" +
		messageBody(source)
	body := strings.TrimSpace(note)
	if body != "" {
		body += "\n\n"
	}
	body += forwarded
	raw, err := buildRawMessage(SendEmailRequest{
		To:      to,
		Subject: ensureSubjectPrefix(h["Subject"], "Fwd:"),
		Body:    body,
	}, nil)
	if err != nil {
		return EmailSummary{}, err
	}
	sent, err := client.gmail.Users.Messages.Send("me", &gmailapi.Message{Raw: raw}).Context(ctx).Do()
	if err != nil {
		return EmailSummary{}, err
	}
	return m.fetchSummary(ctx, client, sent.Id)
}

// CreateDraft creates a Gmail draft. When threadID is non-empty, the draft is
// threaded into the existing conversation with proper In-Reply-To/References.
func (m *Manager) CreateDraft(ctx context.Context, account, threadID string, req SendEmailRequest) (DraftResult, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return DraftResult{}, err
	}

	var extraHeaders map[string]string
	threadID = strings.TrimSpace(threadID)
	if threadID != "" {
		thread, err := client.gmail.Users.Threads.Get("me", threadID).
			Format("metadata").
			MetadataHeaders("Message-ID", "References").
			Context(ctx).
			Do()
		if err != nil {
			return DraftResult{}, err
		}
		if len(thread.Messages) > 0 {
			h := messageHeaders(thread.Messages[len(thread.Messages)-1])
			references := strings.TrimSpace(strings.TrimSpace(h["References"]) + " " + strings.TrimSpace(h["Message-ID"]))
			extraHeaders = map[string]string{
				"In-Reply-To": h["Message-ID"],
				"References":  references,
			}
		}
	}

	raw, err := buildRawMessage(req, extraHeaders)
	if err != nil {
		return DraftResult{}, err
	}
	msg := &gmailapi.Message{Raw: raw}
	if threadID != "" {
		msg.ThreadId = threadID
	}
	d, err := client.gmail.Users.Drafts.Create("me", &gmailapi.Draft{
		Message: msg,
	}).Context(ctx).Do()
	if err != nil {
		return DraftResult{}, err
	}
	result := DraftResult{ID: d.Id}
	if d.Message != nil {
		result.MessageID = d.Message.Id
	}
	return result, nil
}

// ListLabels lists all labels for an account.
func (m *Manager) ListLabels(ctx context.Context, account string) ([]LabelInfo, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	res, err := client.gmail.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	out := make([]LabelInfo, 0, len(res.Labels))
	for _, l := range res.Labels {
		out = append(out, toLabelInfo(l))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// CreateLabel creates a new Gmail label.
func (m *Manager) CreateLabel(ctx context.Context, account, name string) (LabelInfo, error) {
	if strings.TrimSpace(name) == "" {
		return LabelInfo{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return LabelInfo{}, err
	}
	label, err := client.gmail.Users.Labels.Create("me", &gmailapi.Label{Name: name}).Context(ctx).Do()
	if err != nil {
		return LabelInfo{}, err
	}
	return toLabelInfo(label), nil
}

// DeleteLabel deletes a Gmail label by ID.
func (m *Manager) DeleteLabel(ctx context.Context, account, labelID string) error {
	if strings.TrimSpace(labelID) == "" {
		return ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return err
	}
	return client.gmail.Users.Labels.Delete("me", labelID).Context(ctx).Do()
}

// ModifyLabels mutates labels on one message.
func (m *Manager) ModifyLabels(ctx context.Context, account, messageID string, addIDs, removeIDs []string) error {
	if strings.TrimSpace(messageID) == "" {
		return ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return err
	}
	_, err = client.gmail.Users.Messages.Modify("me", messageID, &gmailapi.ModifyMessageRequest{
		AddLabelIds:    addIDs,
		RemoveLabelIds: removeIDs,
	}).Context(ctx).Do()
	return err
}

// ArchiveEmail archives (removes INBOX from) a message.
func (m *Manager) ArchiveEmail(ctx context.Context, account, messageID string) error {
	return m.ModifyLabels(ctx, account, messageID, nil, []string{"INBOX"})
}

// TrashEmail moves a message to trash.
func (m *Manager) TrashEmail(ctx context.Context, account, messageID string) error {
	if strings.TrimSpace(messageID) == "" {
		return ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return err
	}
	_, err = client.gmail.Users.Messages.Trash("me", messageID).Context(ctx).Do()
	return err
}

// MarkEmailRead marks a message as read.
func (m *Manager) MarkEmailRead(ctx context.Context, account, messageID string) error {
	return m.ModifyLabels(ctx, account, messageID, nil, []string{"UNREAD"})
}

// ListCalendars lists calendars for one account.
func (m *Manager) ListCalendars(ctx context.Context, account string) ([]CalendarSummary, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	call := client.calendar.CalendarList.List().Context(ctx).MaxResults(250)
	out := make([]CalendarSummary, 0, 16)
	for {
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, c := range res.Items {
			out = append(out, CalendarSummary{
				ID:       c.Id,
				Summary:  c.Summary,
				TimeZone: c.TimeZone,
				Primary:  c.Primary,
			})
		}
		if res.NextPageToken == "" {
			break
		}
		call = call.PageToken(res.NextPageToken)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Summary < out[j].Summary })
	return out, nil
}

// ListEvents lists events for one calendar.
func (m *Manager) ListEvents(
	ctx context.Context,
	account, calendarID string,
	timeMin, timeMax *time.Time,
	limit int,
	cursor string,
) (CalendarEventsPage, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return CalendarEventsPage{}, err
	}
	calendarID = defaultCalendarID(calendarID)
	limit = normalizeLimit(limit, 25, 250)
	call := client.calendar.Events.List(calendarID).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(int64(limit))
	if timeMin != nil && !timeMin.IsZero() {
		call = call.TimeMin(timeMin.Format(time.RFC3339))
	}
	if timeMax != nil && !timeMax.IsZero() {
		call = call.TimeMax(timeMax.Format(time.RFC3339))
	}
	if cursor != "" {
		call = call.PageToken(cursor)
	}
	events, err := call.Context(ctx).Do()
	if err != nil {
		return CalendarEventsPage{}, err
	}
	return CalendarEventsPage{Items: events.Items, NextCursor: events.NextPageToken}, nil
}

// GetEvent fetches one event by ID.
func (m *Manager) GetEvent(ctx context.Context, account, calendarID, eventID string) (*calendarapi.Event, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	calendarID = defaultCalendarID(calendarID)
	return client.calendar.Events.Get(calendarID, eventID).Context(ctx).Do()
}

// FreeBusy checks busy windows for a set of calendars.
func (m *Manager) FreeBusy(
	ctx context.Context,
	account string,
	calendarIDs []string,
	timeMin, timeMax time.Time,
) (*calendarapi.FreeBusyResponse, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	if len(calendarIDs) == 0 {
		calendarIDs = []string{"primary"}
	}
	if timeMin.IsZero() {
		timeMin = time.Now()
	}
	if timeMax.IsZero() || !timeMax.After(timeMin) {
		timeMax = timeMin.Add(24 * time.Hour)
	}
	items := make([]*calendarapi.FreeBusyRequestItem, 0, len(calendarIDs))
	for _, id := range calendarIDs {
		items = append(items, &calendarapi.FreeBusyRequestItem{Id: id})
	}
	return client.calendar.Freebusy.Query(&calendarapi.FreeBusyRequest{
		TimeMin: timeMin.Format(time.RFC3339),
		TimeMax: timeMax.Format(time.RFC3339),
		Items:   items,
	}).Context(ctx).Do()
}

// CreateEvent creates an event.
func (m *Manager) CreateEvent(ctx context.Context, account, calendarID string, event *calendarapi.Event) (*calendarapi.Event, error) {
	if event == nil {
		return nil, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	calendarID = defaultCalendarID(calendarID)
	return client.calendar.Events.Insert(calendarID, event).Context(ctx).Do()
}

// UpdateEvent patches an existing event.
func (m *Manager) UpdateEvent(ctx context.Context, account, calendarID, eventID string, event *calendarapi.Event) (*calendarapi.Event, error) {
	if strings.TrimSpace(eventID) == "" || event == nil {
		return nil, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	calendarID = defaultCalendarID(calendarID)
	return client.calendar.Events.Patch(calendarID, eventID, event).Context(ctx).Do()
}

// DeleteEvent deletes an event.
func (m *Manager) DeleteEvent(ctx context.Context, account, calendarID, eventID string) error {
	if strings.TrimSpace(eventID) == "" {
		return ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return err
	}
	calendarID = defaultCalendarID(calendarID)
	return client.calendar.Events.Delete(calendarID, eventID).Context(ctx).Do()
}

// RSVPEvent updates the caller's attendee status on an event.
func (m *Manager) RSVPEvent(ctx context.Context, account, calendarID, eventID, response string) (*calendarapi.Event, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, ErrInvalidInput
	}
	response = strings.ToLower(strings.TrimSpace(response))
	switch response {
	case "accepted", "declined", "tentative":
	default:
		return nil, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return nil, err
	}
	calendarID = defaultCalendarID(calendarID)
	event, err := client.calendar.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	found := false
	for _, attendee := range event.Attendees {
		if attendee.Self || strings.EqualFold(attendee.Email, client.email) {
			attendee.ResponseStatus = response
			found = true
		}
	}
	if !found && client.email != "" {
		event.Attendees = append(event.Attendees, &calendarapi.EventAttendee{
			Email:          client.email,
			ResponseStatus: response,
			Self:           true,
		})
	}
	return client.calendar.Events.Patch(calendarID, eventID, &calendarapi.Event{
		Attendees: event.Attendees,
	}).Context(ctx).Do()
}

// ListDriveFiles lists Drive files for one account.
func (m *Manager) ListDriveFiles(ctx context.Context, account, query string, limit int, cursor string) (DriveFilesResult, error) {
	client, err := m.Resolve(account)
	if err != nil {
		return DriveFilesResult{}, err
	}
	limit = normalizeLimit(limit, 30, 200)
	if strings.TrimSpace(query) == "" {
		query = "trashed = false"
	}
	call := client.drive.Files.List().
		Q(query).
		PageSize(int64(limit)).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Fields("nextPageToken,files(id,name,mimeType,modifiedTime,webViewLink,size,parents)")
	if cursor != "" {
		call = call.PageToken(cursor)
	}
	res, err := call.Context(ctx).Do()
	if err != nil {
		return DriveFilesResult{}, err
	}
	out := DriveFilesResult{
		Items:      make([]DriveFileSummary, 0, len(res.Files)),
		NextCursor: res.NextPageToken,
	}
	for _, file := range res.Files {
		out.Items = append(out.Items, toDriveFileSummary(file))
	}
	return out, nil
}

// GetDriveFile returns Drive metadata without downloading file contents.
func (m *Manager) GetDriveFile(ctx context.Context, account, fileID string) (DriveFileSummary, error) {
	if strings.TrimSpace(fileID) == "" {
		return DriveFileSummary{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return DriveFileSummary{}, err
	}
	meta, err := client.drive.Files.Get(fileID).
		SupportsAllDrives(true).
		Fields("id,name,mimeType,modifiedTime,webViewLink,size,parents").
		Context(ctx).
		Do()
	if err != nil {
		return DriveFileSummary{}, err
	}
	return toDriveFileSummary(meta), nil
}

// ReadDriveFile reads one Drive file's contents with safe size limits.
func (m *Manager) ReadDriveFile(ctx context.Context, account, fileID, exportMIME string, maxBytes int) (DriveFileContent, error) {
	if strings.TrimSpace(fileID) == "" {
		return DriveFileContent{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return DriveFileContent{}, err
	}
	maxBytes = normalizeLimit(maxBytes, maxDriveReadBytes, 2*maxDriveReadBytes)
	meta, err := client.drive.Files.Get(fileID).
		SupportsAllDrives(true).
		Fields("id,name,mimeType,modifiedTime,webViewLink,size,parents").
		Context(ctx).
		Do()
	if err != nil {
		return DriveFileContent{}, err
	}
	if meta.MimeType == "application/vnd.google-apps.folder" {
		return DriveFileContent{}, ErrInvalidInput
	}

	body, err := downloadDriveBody(ctx, client.drive, meta, exportMIME)
	if err != nil {
		return DriveFileContent{}, err
	}
	defer body.Close()

	raw, truncated, err := readLimited(body, maxBytes)
	if err != nil {
		return DriveFileContent{}, err
	}

	out := DriveFileContent{
		File:      toDriveFileSummary(meta),
		Truncated: truncated,
	}
	if utf8.Valid(raw) {
		out.Content = string(raw)
		out.Encoding = "utf-8"
		return out, nil
	}
	out.Content = base64.StdEncoding.EncodeToString(raw)
	out.Encoding = "base64"
	return out, nil
}

// CreateDriveFile creates a Drive file in one account.
func (m *Manager) CreateDriveFile(ctx context.Context, account string, req DriveWriteRequest) (DriveFileSummary, error) {
	if strings.TrimSpace(req.Name) == "" {
		return DriveFileSummary{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return DriveFileSummary{}, err
	}
	metadata := &driveapi.File{
		Name:    req.Name,
		Parents: req.Parents,
	}
	if strings.TrimSpace(req.MimeType) != "" {
		metadata.MimeType = strings.TrimSpace(req.MimeType)
	}
	uploadContentType := firstNonEmpty(strings.TrimSpace(req.MimeType), "text/plain")
	if req.AsGoogleDoc {
		metadata.MimeType = "application/vnd.google-apps.document"
		// Keep the upload's content type as the SOURCE format (e.g. text/html)
		// so Drive CONVERTS it into a formatted Doc. Forcing text/plain here
		// made Drive ingest the HTML literally, so the Doc showed raw <h1>/<ul>
		// tags instead of formatted headings/lists.
	}
	call := client.drive.Files.Create(metadata).
		SupportsAllDrives(true).
		Fields("id,name,mimeType,modifiedTime,webViewLink,size,parents")
	if strings.TrimSpace(req.Content) != "" {
		call = call.Media(strings.NewReader(req.Content), googleapi.ContentType(uploadContentType))
	}
	created, err := call.Context(ctx).Do()
	if err != nil {
		return DriveFileSummary{}, err
	}
	return toDriveFileSummary(created), nil
}

// UpdateDriveFile updates metadata/content for one Drive file.
func (m *Manager) UpdateDriveFile(ctx context.Context, account, fileID string, req DriveUpdateRequest) (DriveFileSummary, error) {
	if strings.TrimSpace(fileID) == "" {
		return DriveFileSummary{}, ErrInvalidInput
	}
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(req.MimeType) == "" && strings.TrimSpace(req.Content) == "" {
		return DriveFileSummary{}, ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return DriveFileSummary{}, err
	}
	metadata := &driveapi.File{}
	if req.Name != "" {
		metadata.Name = req.Name
	}
	// req.MimeType is the SOURCE content type of the uploaded bytes (e.g.
	// text/html), NOT a new file type. Only set it as the file's target
	// mimeType when it's a Google-native type (a deliberate conversion
	// target). Setting a source type like text/html as an existing Google
	// Doc's mimeType turned it into a raw HTML file — which is why synced
	// docs rendered literal HTML tags. Otherwise it's used solely as the
	// upload media content type below, so Drive converts into the existing Doc.
	if strings.HasPrefix(strings.TrimSpace(req.MimeType), "application/vnd.google-apps.") {
		metadata.MimeType = strings.TrimSpace(req.MimeType)
	}
	call := client.drive.Files.Update(fileID, metadata).
		SupportsAllDrives(true).
		Fields("id,name,mimeType,modifiedTime,webViewLink,size,parents")
	if strings.TrimSpace(req.Content) != "" {
		uploadContentType := firstNonEmpty(strings.TrimSpace(req.MimeType), "text/plain")
		call = call.Media(strings.NewReader(req.Content), googleapi.ContentType(uploadContentType))
	}
	updated, err := call.Context(ctx).Do()
	if err != nil {
		return DriveFileSummary{}, err
	}
	return toDriveFileSummary(updated), nil
}

// DeleteDriveFile removes one Drive file.
func (m *Manager) DeleteDriveFile(ctx context.Context, account, fileID string) error {
	if strings.TrimSpace(fileID) == "" {
		return ErrInvalidInput
	}
	client, err := m.Resolve(account)
	if err != nil {
		return err
	}
	return client.drive.Files.Delete(fileID).SupportsAllDrives(true).Context(ctx).Do()
}

// UnifiedDriveSearch searches Drive across all authenticated accounts.
func (m *Manager) UnifiedDriveSearch(ctx context.Context, query string, limit int) ([]AccountDriveFile, error) {
	limit = normalizeLimit(limit, 30, 300)
	accounts := m.authenticatedAccounts()
	if len(accounts) == 0 {
		return nil, ErrAccountNotAuthenticated
	}

	perAccount := limit
	if len(accounts) > 1 {
		perAccount = minInt(maxInt(limit/len(accounts), 5), 100)
	}

	out := make([]AccountDriveFile, 0, limit)
	for _, ac := range accounts {
		page, err := m.ListDriveFiles(ctx, ac.alias, query, perAccount, "")
		if err != nil {
			continue
		}
		for _, file := range page.Items {
			out = append(out, AccountDriveFile{
				Account: ac.alias,
				Email:   ac.email,
				File:    file,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return driveModifiedUnix(out[i].File) > driveModifiedUnix(out[j].File)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UnifiedInbox returns unread inbox messages across all accounts.
func (m *Manager) UnifiedInbox(ctx context.Context, limit int) ([]AccountEmail, error) {
	return m.UnifiedSearch(ctx, "in:inbox is:unread", limit)
}

// UnifiedSearch runs one Gmail query across all authenticated accounts.
func (m *Manager) UnifiedSearch(ctx context.Context, query string, limit int) ([]AccountEmail, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidInput
	}
	limit = normalizeLimit(limit, 30, 200)
	accounts := m.authenticatedAccounts()
	if len(accounts) == 0 {
		return nil, ErrAccountNotAuthenticated
	}

	perAccount := limit
	if len(accounts) > 1 {
		perAccount = minInt(maxInt(limit/len(accounts), 5), 50)
	}

	out := make([]AccountEmail, 0, limit)
	for _, ac := range accounts {
		page, err := m.SearchEmail(ctx, ac.alias, query, perAccount, "")
		if err != nil {
			continue
		}
		for _, msg := range page.Items {
			out = append(out, AccountEmail{
				Account: ac.alias,
				Email:   ac.email,
				Message: msg,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Message.InternalDate > out[j].Message.InternalDate
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UnifiedAgenda returns merged calendar events across all accounts/calendars.
func (m *Manager) UnifiedAgenda(ctx context.Context, timeMin, timeMax time.Time, limit int) ([]AccountEvent, error) {
	limit = normalizeLimit(limit, 100, 1000)
	if timeMin.IsZero() {
		timeMin = time.Now()
	}
	if timeMax.IsZero() || !timeMax.After(timeMin) {
		timeMax = timeMin.Add(7 * 24 * time.Hour)
	}
	accounts := m.authenticatedAccounts()
	if len(accounts) == 0 {
		return nil, ErrAccountNotAuthenticated
	}

	out := make([]AccountEvent, 0, limit)
	for _, ac := range accounts {
		calendars, err := m.ListCalendars(ctx, ac.alias)
		if err != nil {
			continue
		}
		for _, cal := range calendars {
			events, err := m.ListEvents(ctx, ac.alias, cal.ID, &timeMin, &timeMax, 250, "")
			if err != nil {
				continue
			}
			for _, event := range events.Items {
				out = append(out, AccountEvent{
					Account:      ac.alias,
					Email:        ac.email,
					CalendarID:   cal.ID,
					CalendarName: cal.Summary,
					Event:        event,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return eventStartUnix(out[i].Event) < eventStartUnix(out[j].Event)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Manager) authenticatedAccounts() []*managedAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*managedAccount, 0, len(m.accounts))
	for _, ac := range m.accounts {
		if ac.client != nil {
			out = append(out, ac)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].alias < out[j].alias })
	return out
}

func (m *Manager) resolveAccountLocked(account string) (*managedAccount, error) {
	key := strings.ToLower(strings.TrimSpace(account))
	if key == "" {
		if m.cfg.DefaultAccount != "" {
			key = normalizeAlias(m.cfg.DefaultAccount)
		} else if len(m.accounts) == 1 {
			for alias := range m.accounts {
				key = alias
			}
		}
	}
	if key == "" {
		return nil, ErrInvalidInput
	}
	if ac := m.accounts[key]; ac != nil {
		return ac, nil
	}
	if ac := m.emails[key]; ac != nil {
		return ac, nil
	}
	return nil, ErrAccountNotFound
}

func (m *Manager) ensureOAuthConfig() (*oauth2.Config, error) {
	m.mu.RLock()
	if m.oauthConfig != nil {
		cfg := *m.oauthConfig
		m.mu.RUnlock()
		return &cfg, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.oauthConfig != nil {
		cfg := *m.oauthConfig
		return &cfg, nil
	}
	cfg, err := loadOAuth2Config(m.oauthKeysPath, defaultGoogleScopes)
	if err != nil {
		return nil, err
	}
	m.oauthConfig = cfg
	copyCfg := *cfg
	return &copyCfg, nil
}

func (m *Manager) fetchSummary(ctx context.Context, client *Client, messageID string) (EmailSummary, error) {
	msg, err := client.gmail.Users.Messages.Get("me", messageID).
		Format("metadata").
		MetadataHeaders("Subject", "From", "Date").
		Context(ctx).
		Do()
	if err != nil {
		return EmailSummary{}, err
	}
	return toEmailSummary(msg), nil
}

func newClient(
	ctx context.Context,
	oauthCfg *oauth2.Config,
	alias, email, tokenPath string,
	token *oauth2.Token,
) (*Client, error) {
	if oauthCfg == nil {
		return nil, ErrOAuthKeysNotFound
	}
	if token == nil {
		return nil, ErrAccountNotAuthenticated
	}
	baseSource := oauthCfg.TokenSource(ctx, token)
	persisting := &persistingTokenSource{
		tokenSource: baseSource,
		tokenPath:   tokenPath,
		last:        cloneToken(token),
	}
	httpClient := oauth2.NewClient(ctx, persisting)
	gmailSvc, err := gmailapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new gmail service: %w", err)
	}
	calSvc, err := calendarapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new calendar service: %w", err)
	}
	driveSvc, err := driveapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("new drive service: %w", err)
	}
	return &Client{
		alias:       alias,
		email:       email,
		gmail:       gmailSvc,
		calendar:    calSvc,
		drive:       driveSvc,
		tokenSource: persisting,
	}, nil
}

type persistingTokenSource struct {
	mu          sync.Mutex
	tokenSource oauth2.TokenSource
	tokenPath   string
	last        *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last == nil || p.last.AccessToken != tok.AccessToken || p.last.RefreshToken != tok.RefreshToken || !p.last.Expiry.Equal(tok.Expiry) {
		_ = saveToken(p.tokenPath, tok)
		p.last = cloneToken(tok)
	}
	return tok, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	var tok oauth2.Token
	if err := readJSONFile(path, &tok); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tok.AccessToken) == "" && strings.TrimSpace(tok.RefreshToken) == "" {
		return nil, fmt.Errorf("token file %s has no access_token or refresh_token", path)
	}
	return &tok, nil
}

func saveToken(path string, tok *oauth2.Token) error {
	if tok == nil {
		return ErrInvalidInput
	}
	return writeJSONFile(path, tok, 0o600)
}

func cloneToken(tok *oauth2.Token) *oauth2.Token {
	if tok == nil {
		return nil
	}
	out := *tok
	return &out
}

var headerSanitizer = strings.NewReplacer("\r", "", "\n", "")

func sanitizeHeader(v string) string {
	return headerSanitizer.Replace(strings.TrimSpace(v))
}

func sanitizeAddresses(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		s := sanitizeHeader(a)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func buildRawMessage(req SendEmailRequest, extraHeaders map[string]string) (string, error) {
	if len(req.To) == 0 {
		return "", ErrInvalidInput
	}
	if strings.TrimSpace(req.Body) == "" && strings.TrimSpace(req.HTMLBody) == "" {
		return "", ErrInvalidInput
	}

	var b strings.Builder
	b.WriteString("To: " + strings.Join(sanitizeAddresses(req.To), ", ") + "\r\n")
	if len(req.Cc) > 0 {
		b.WriteString("Cc: " + strings.Join(sanitizeAddresses(req.Cc), ", ") + "\r\n")
	}
	if len(req.Bcc) > 0 {
		b.WriteString("Bcc: " + strings.Join(sanitizeAddresses(req.Bcc), ", ") + "\r\n")
	}
	b.WriteString("Subject: " + sanitizeHeader(req.Subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	for k, v := range extraHeaders {
		sv := sanitizeHeader(v)
		if sv != "" {
			b.WriteString(sanitizeHeader(k) + ": " + sv + "\r\n")
		}
	}

	contentType := "text/plain; charset=UTF-8"
	body := req.Body
	if strings.TrimSpace(body) == "" {
		contentType = "text/html; charset=UTF-8"
		body = req.HTMLBody
	}
	b.WriteString("Content-Type: " + contentType + "\r\n\r\n")
	b.WriteString(body)

	return base64.RawURLEncoding.EncodeToString([]byte(b.String())), nil
}

func toEmailSummary(msg *gmailapi.Message) EmailSummary {
	h := messageHeaders(msg)
	return EmailSummary{
		ID:           msg.Id,
		ThreadID:     msg.ThreadId,
		Snippet:      msg.Snippet,
		Subject:      h["Subject"],
		From:         h["From"],
		Date:         h["Date"],
		InternalDate: msg.InternalDate,
		LabelIDs:     append([]string(nil), msg.LabelIds...),
	}
}

func messageHeaders(msg *gmailapi.Message) map[string]string {
	out := map[string]string{}
	if msg == nil || msg.Payload == nil {
		return out
	}
	for _, header := range msg.Payload.Headers {
		if header == nil || header.Name == "" {
			continue
		}
		out[header.Name] = header.Value
	}
	return out
}

func messageBody(msg *gmailapi.Message) string {
	if msg == nil || msg.Payload == nil {
		return ""
	}
	return strings.TrimSpace(decodePart(msg.Payload))
}

func decodePart(part *gmailapi.MessagePart) string {
	if part == nil {
		return ""
	}
	for _, sub := range part.Parts {
		switch {
		case strings.HasPrefix(sub.MimeType, "text/plain"):
			if body := decodeBodyData(sub.Body); body != "" {
				return body
			}
		case strings.HasPrefix(sub.MimeType, "text/html"):
			if body := decodeBodyData(sub.Body); body != "" {
				return body
			}
		default:
			if nested := decodePart(sub); nested != "" {
				return nested
			}
		}
	}
	return decodeBodyData(part.Body)
}

func decodeBodyData(body *gmailapi.MessagePartBody) string {
	if body == nil || body.Data == "" {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(body.Data)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(body.Data)
	}
	if err != nil {
		return ""
	}
	return string(raw)
}

func toLabelInfo(label *gmailapi.Label) LabelInfo {
	if label == nil {
		return LabelInfo{}
	}
	return LabelInfo{
		ID:                    label.Id,
		Name:                  label.Name,
		Type:                  label.Type,
		MessageListVisibility: label.MessageListVisibility,
		LabelListVisibility:   label.LabelListVisibility,
	}
}

func defaultCalendarID(calendarID string) string {
	if strings.TrimSpace(calendarID) == "" {
		return "primary"
	}
	return calendarID
}

func firstEmailAddress(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return ""
	}
	addrs, err := mail.ParseAddressList(headerValue)
	if err == nil && len(addrs) > 0 {
		return addrs[0].Address
	}
	return headerValue
}

func ensureSubjectPrefix(subject, prefix string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return strings.TrimSpace(prefix)
	}
	if strings.HasPrefix(strings.ToLower(subject), strings.ToLower(prefix)) {
		return subject
	}
	return strings.TrimSpace(prefix) + " " + subject
}

func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func normalizeLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func eventStartUnix(event *calendarapi.Event) int64 {
	if event == nil || event.Start == nil {
		return 0
	}
	if event.Start.DateTime != "" {
		if ts, err := time.Parse(time.RFC3339, event.Start.DateTime); err == nil {
			return ts.Unix()
		}
	}
	if event.Start.Date != "" {
		if ts, err := time.Parse("2006-01-02", event.Start.Date); err == nil {
			return ts.Unix()
		}
	}
	return 0
}

func driveModifiedUnix(file DriveFileSummary) int64 {
	if strings.TrimSpace(file.ModifiedTime) == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, file.ModifiedTime)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}

func downloadDriveBody(ctx context.Context, svc *driveapi.Service, file *driveapi.File, exportMIME string) (io.ReadCloser, error) {
	if file == nil || strings.TrimSpace(file.Id) == "" {
		return nil, ErrInvalidInput
	}
	if strings.HasPrefix(file.MimeType, "application/vnd.google-apps.") {
		mime := strings.TrimSpace(exportMIME)
		if mime == "" {
			mime = defaultDriveExportMIME(file.MimeType)
		}
		resp, err := svc.Files.Export(file.Id, mime).Context(ctx).Download()
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	}
	resp, err := svc.Files.Get(file.Id).SupportsAllDrives(true).Context(ctx).Download()
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func defaultDriveExportMIME(googleMime string) string {
	switch googleMime {
	case "application/vnd.google-apps.document":
		return "text/plain"
	case "application/vnd.google-apps.spreadsheet":
		return "text/csv"
	case "application/vnd.google-apps.presentation":
		return "text/plain"
	default:
		return "text/plain"
	}
}

func readLimited(r io.Reader, max int) ([]byte, bool, error) {
	if max <= 0 {
		max = maxDriveReadBytes
	}
	reader := &io.LimitedReader{R: r, N: int64(max + 1)}
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	if len(buf) > max {
		return buf[:max], true, nil
	}
	return buf, false, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
