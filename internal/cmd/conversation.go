package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/lock"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// -----------------------------------------------------------------------------
// Plural: `chatwoot convs` — list conversations.
// -----------------------------------------------------------------------------

type ConvsCmd struct {
	Status   string   `short:"s" default:"open" help:"Filter: open, resolved, pending, snoozed."`
	Inbox    int      `short:"i" help:"Filter by inbox ID."`
	Assignee string   `default:"me" help:"Filter: me, unassigned, all."`
	Team     int      `help:"Filter by team ID."`
	Label    []string `short:"l" help:"Filter by labels."`
	Query    string   `help:"Search conversations by message content."`
	Page     int      `short:"p" default:"1" help:"Page number."`
}

func (c *ConvsCmd) Run(app *App) error {
	resp, err := app.Client.Conversations().List(sdk.ListOptions{
		Status:       c.Status,
		InboxID:      c.Inbox,
		AssigneeType: c.Assignee,
		TeamID:       c.Team,
		Query:        c.Query,
		Labels:       c.Label,
		Page:         c.Page,
	})
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	convos := resp.Data.Payload
	if len(convos) == 0 {
		fmt.Println("No conversations found.")
		return nil
	}

	headers := []string{"ID", "Status", "Contact", "Assignee", "Inbox", "Labels", "Last Activity"}
	rows := make([][]string, 0, len(convos))
	for _, conv := range convos {
		rows = append(rows, []string{
			strconv.Itoa(conv.ID),
			conv.Status,
			senderName(conv),
			assigneeName(conv),
			conv.Meta.Channel,
			labelsString(conv.Labels),
			formatTimestamp(conv.LastActivityAt),
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

// -----------------------------------------------------------------------------
// Singular: `chatwoot conv <verb> <id> ...` — context for one conversation.
// -----------------------------------------------------------------------------

type ConvCmd struct {
	View     ConvViewCmd     `cmd:"" default:"withargs" help:"View a conversation (default)."`
	Messages ConvMessagesCmd `cmd:"" help:"List messages in the conversation."`
	Reply    ConvReplyCmd    `cmd:"" help:"Reply to the conversation."`
	Resolve  ConvResolveCmd  `cmd:"" help:"Mark the conversation as resolved."`
	Open     ConvOpenCmd     `cmd:"" help:"Set the conversation status to open."`
	Pending  ConvPendingCmd  `cmd:"" help:"Set the conversation status to pending."`
	Snooze   ConvSnoozeCmd   `cmd:"" help:"Snooze the conversation (default: until next reply)."`
	Assign   ConvAssignCmd   `cmd:"" help:"Assign the conversation to an agent and/or team (--agent, --team)."`
	Unassign ConvUnassignCmd `cmd:"" help:"Remove the assignee."`
	Label    ConvLabelCmd    `cmd:"" help:"Set labels on the conversation (replaces existing)."`
	Priority ConvPriorityCmd `cmd:"" help:"Set or clear conversation priority."`
	Contact  ConvContactCmd  `cmd:"" help:"View the contact (sender) for the conversation."`
}

// -- view ---------------------------------------------------------------------

type ConvViewCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvViewCmd) Run(app *App) error {
	conv, err := app.Client.Conversations().Get(c.ID)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(conv)
		return nil
	}

	sender := ""
	if conv.Meta.Sender != nil {
		sender = conv.Meta.Sender.Name
		if conv.Meta.Sender.Email != "" {
			sender += " <" + conv.Meta.Sender.Email + ">"
		}
	}
	team := ""
	if conv.Meta.Team != nil {
		team = conv.Meta.Team.Name
	}
	priority := "none"
	if conv.Priority != nil {
		priority = *conv.Priority
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "ID", Value: strconv.Itoa(conv.ID)},
		{Key: "Status", Value: conv.Status},
		{Key: "Priority", Value: priority},
		{Key: "Contact", Value: sender},
		{Key: "Assignee", Value: assigneeName(*conv)},
		{Key: "Team", Value: team},
		{Key: "Channel", Value: conv.Meta.Channel},
		{Key: "Labels", Value: labelsString(conv.Labels)},
		{Key: "Messages", Value: strconv.Itoa(conv.MessagesCount)},
		{Key: "Created", Value: formatTimestamp(conv.CreatedAt)},
		{Key: "Last Activity", Value: formatTimestamp(conv.LastActivityAt)},
	})
	return nil
}

// -- messages -----------------------------------------------------------------

type ConvMessagesCmd struct {
	ID     int `arg:"" help:"Conversation ID."`
	Before int `help:"Messages before this message ID."`
	After  int `help:"Messages after this message ID."`
}

func (c *ConvMessagesCmd) Run(app *App) error {
	resp, err := app.Client.Messages(c.ID).List(c.Before, c.After)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No messages found.")
		return nil
	}

	headers := []string{"ID", "Type", "Sender", "Content", "Time"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, msg := range resp.Payload {
		sender := ""
		if msg.Sender != nil {
			sender = msg.Sender.Name
		}
		msgType := messageTypeName(msg.MessageType)
		if msg.Private {
			msgType = "note"
		}
		rows = append(rows, []string{
			strconv.Itoa(msg.ID),
			msgType,
			sender,
			truncate(strings.ReplaceAll(msg.Content, "\n", " "), 60),
			formatTimestamp(msg.CreatedAt),
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

// -- per-conversation lock ----------------------------------------------------

// withConvLock runs fn while holding the per-conversation lock, so concurrent
// chatwoot processes can't mutate the same conversation at once. If another
// process holds the lock, it fails fast instead of waiting: a queued duplicate
// would still fire after the holder finishes, which is exactly what the lock
// exists to prevent.
func withConvLock(id int, fn func() error) error {
	lk, err := lock.AcquireConversation(id)
	if err != nil {
		if errors.Is(err, lock.ErrLocked) {
			return fmt.Errorf("conversation %d: another chatwoot command is already running on this conversation; try again in a moment", id)
		}
		return err
	}
	defer lk.Release()
	return fn()
}

// -- reply --------------------------------------------------------------------

type ConvReplyCmd struct {
	ID      int    `arg:"" help:"Conversation ID."`
	Text    string `arg:"" help:"Reply content."`
	Private bool   `help:"Send as a private note instead of a public reply."`
}

func (c *ConvReplyCmd) Run(app *App) error {
	return withConvLock(c.ID, func() error {
		msg, err := app.Client.Messages(c.ID).Create(c.Text, c.Private)
		if err != nil {
			return err
		}
		if app.Printer.Quiet {
			fmt.Println(msg.ID)
			return nil
		}
		kind := "reply"
		if c.Private {
			kind = "note"
		}
		fmt.Printf("Sent %s on conversation %d (message %d).\n", kind, c.ID, msg.ID)
		return nil
	})
}

// -- status verbs: resolve / open / pending / snooze --------------------------

type ConvResolveCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvResolveCmd) Run(app *App) error {
	return withConvLock(c.ID, func() error {
		return setStatus(app, c.ID, "resolved", nil)
	})
}

type ConvOpenCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvOpenCmd) Run(app *App) error {
	return withConvLock(c.ID, func() error {
		return setStatus(app, c.ID, "open", nil)
	})
}

type ConvPendingCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvPendingCmd) Run(app *App) error {
	return withConvLock(c.ID, func() error {
		return setStatus(app, c.ID, "pending", nil)
	})
}

type ConvSnoozeCmd struct {
	ID    int    `arg:"" help:"Conversation ID."`
	Until string `help:"Snooze duration (e.g. 24h, 7d) or absolute date (2006-01-02). Omit to snooze until next reply."`
}

func (c *ConvSnoozeCmd) Run(app *App) error {
	var until *int64
	if c.Until != "" {
		ts, err := parseSnoozeUntil(c.Until)
		if err != nil {
			return err
		}
		until = &ts
	}
	return withConvLock(c.ID, func() error {
		return setStatus(app, c.ID, "snoozed", until)
	})
}

func setStatus(app *App, id int, status string, snoozedUntil *int64) error {
	if _, err := app.Client.Conversations().ToggleStatus(id, status, snoozedUntil); err != nil {
		return err
	}
	if app.Printer.Quiet {
		fmt.Println(id)
		return nil
	}
	if snoozedUntil != nil {
		fmt.Printf("Conversation %d -> %s (until %s).\n", id, status, formatTimestamp(*snoozedUntil))
		return nil
	}
	fmt.Printf("Conversation %d -> %s.\n", id, status)
	return nil
}

func parseSnoozeUntil(s string) (int64, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(d).Unix(), nil
	}
	if d, ok := parseExtendedDuration(s); ok {
		return time.Now().Add(d).Unix(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("invalid --until %q (try 24h, 7d, or 2026-05-10)", s)
}

func parseExtendedDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, false
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, true
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, true
	}
	return 0, false
}

// -- assign / unassign --------------------------------------------------------

type ConvAssignCmd struct {
	ID    int    `arg:"" help:"Conversation ID."`
	Agent string `help:"Agent ID, 'me', or name (case-insensitive substring)."`
	Team  int    `help:"Team ID."`
}

func (c *ConvAssignCmd) Run(app *App) error {
	if c.Agent == "" && c.Team == 0 {
		return fmt.Errorf("--agent or --team required")
	}
	var agentPtr *int
	if c.Agent != "" {
		id, err := resolveAgent(app, c.Agent)
		if err != nil {
			return err
		}
		agentPtr = &id
	}
	if err := withConvLock(c.ID, func() error {
		_, err := app.Client.Conversations().Assign(c.ID, agentPtr, c.Team)
		return err
	}); err != nil {
		return err
	}
	if app.Printer.Quiet {
		fmt.Println(c.ID)
		return nil
	}
	switch {
	case agentPtr != nil && c.Team > 0:
		fmt.Printf("Conversation %d assigned to agent %d, team %d.\n", c.ID, *agentPtr, c.Team)
	case agentPtr != nil:
		fmt.Printf("Conversation %d assigned to agent %d.\n", c.ID, *agentPtr)
	default:
		fmt.Printf("Conversation %d assigned to team %d.\n", c.ID, c.Team)
	}
	return nil
}

type ConvUnassignCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvUnassignCmd) Run(app *App) error {
	if err := withConvLock(c.ID, func() error {
		return app.Client.Conversations().Unassign(c.ID)
	}); err != nil {
		return err
	}
	if app.Printer.Quiet {
		fmt.Println(c.ID)
		return nil
	}
	fmt.Printf("Conversation %d unassigned.\n", c.ID)
	return nil
}

// -- label --------------------------------------------------------------------

type ConvLabelCmd struct {
	ID     int      `arg:"" help:"Conversation ID."`
	Labels []string `arg:"" help:"Labels to set (comma-separated or repeated). Replaces existing."`
}

func (c *ConvLabelCmd) Run(app *App) error {
	flat := make([]string, 0, len(c.Labels))
	for _, raw := range c.Labels {
		for _, part := range strings.Split(raw, ",") {
			if t := strings.TrimSpace(part); t != "" {
				flat = append(flat, t)
			}
		}
	}
	if err := withConvLock(c.ID, func() error {
		_, err := app.Client.Labels(c.ID).Add(flat)
		return err
	}); err != nil {
		return err
	}
	if app.Printer.Quiet {
		fmt.Println(c.ID)
		return nil
	}
	fmt.Printf("Conversation %d labels: %s\n", c.ID, strings.Join(flat, ", "))
	return nil
}

// -- priority -----------------------------------------------------------------

type ConvPriorityCmd struct {
	ID    int    `arg:"" help:"Conversation ID."`
	Level string `arg:"" enum:"urgent,high,medium,low,none" help:"Priority: urgent, high, medium, low, or none."`
}

func (c *ConvPriorityCmd) Run(app *App) error {
	value := c.Level
	if value == "none" {
		value = ""
	}
	if err := withConvLock(c.ID, func() error {
		return app.Client.Conversations().UpdatePriority(c.ID, value)
	}); err != nil {
		return err
	}
	if app.Printer.Quiet {
		fmt.Println(c.ID)
		return nil
	}
	fmt.Printf("Conversation %d priority -> %s.\n", c.ID, c.Level)
	return nil
}

// -- contact ------------------------------------------------------------------

type ConvContactCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConvContactCmd) Run(app *App) error {
	conv, err := app.Client.Conversations().Get(c.ID)
	if err != nil {
		return err
	}
	if conv.Meta.Sender == nil {
		return fmt.Errorf("conversation %d has no associated contact", c.ID)
	}
	contact, err := app.Client.Contacts().Get(conv.Meta.Sender.ID)
	if err != nil {
		return err
	}
	return renderContact(app, contact)
}

// -----------------------------------------------------------------------------
// Helpers.
// -----------------------------------------------------------------------------

func resolveAgent(app *App, ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, fmt.Errorf("agent reference required")
	}
	if strings.EqualFold(ref, "me") {
		if app.Config != nil && app.Config.UserID != 0 {
			return app.Config.UserID, nil
		}
		profile, err := app.Client.Profile().Get()
		if err != nil {
			return 0, fmt.Errorf("cannot resolve 'me': %w", err)
		}
		if app.Config != nil {
			app.Config.UserID = profile.ID
			_ = config.Save(app.Config)
		}
		return profile.ID, nil
	}
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	agents, err := app.Client.Agents().List()
	if err != nil {
		return 0, fmt.Errorf("agent lookup failed: %w", err)
	}
	needle := strings.ToLower(ref)
	matches := make([]sdk.AgentFull, 0, 2)
	for _, a := range agents {
		if strings.Contains(strings.ToLower(a.Name), needle) || strings.HasPrefix(strings.ToLower(a.Email), needle) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no agent matched %q", ref)
	case 1:
		return matches[0].ID, nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = fmt.Sprintf("%s (%d)", m.Name, m.ID)
		}
		return 0, fmt.Errorf("multiple agents matched %q: %s", ref, strings.Join(names, ", "))
	}
}

func senderName(conv sdk.Conversation) string {
	if conv.Meta.Sender == nil {
		return ""
	}
	return conv.Meta.Sender.Name
}

func assigneeName(conv sdk.Conversation) string {
	if conv.Meta.Assignee == nil {
		return ""
	}
	return conv.Meta.Assignee.Name
}

func labelsString(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return strings.Join(labels, ", ")
}

func messageTypeName(t int) string {
	switch t {
	case 0:
		return "incoming"
	case 1:
		return "outgoing"
	case 2:
		return "activity"
	default:
		return strconv.Itoa(t)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}
