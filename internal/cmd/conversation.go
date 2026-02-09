package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/chatwoot/chatwoot-cli/internal/output"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

type ConversationCmd struct {
	List ConversationListCmd `cmd:"" default:"1" help:"List conversations."`
	View ConversationViewCmd `cmd:"" help:"View a conversation."`
}

type ConversationListCmd struct {
	Status   string   `short:"s" default:"open" help:"Filter: open, resolved, pending, snoozed."`
	Inbox    int      `short:"i" help:"Filter by inbox ID."`
	Assignee string   `default:"me" help:"Filter: me, unassigned, all."`
	Team     int      `help:"Filter by team ID."`
	Label    []string `short:"l" help:"Filter by labels."`
	Sort     string   `default:"latest" help:"Sort: latest, created_at, priority."`
	Page     int      `short:"p" default:"1" help:"Page number."`
}

func (c *ConversationListCmd) Run(app *App) error {
	resp, err := app.Client.Conversations().List(sdk.ListOptions{
		Status:       c.Status,
		InboxID:      c.Inbox,
		AssigneeType: c.Assignee,
		TeamID:       c.Team,
		Labels:       c.Label,
		SortBy:       c.Sort,
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
		sender := ""
		if conv.Meta.Sender != nil {
			sender = conv.Meta.Sender.Name
		}
		assignee := ""
		if conv.Meta.Assignee != nil {
			assignee = conv.Meta.Assignee.Name
		}
		inboxName := ""
		if conv.Meta.Channel != "" {
			inboxName = conv.Meta.Channel
		}
		labels := ""
		if len(conv.Labels) > 0 {
			labels = fmt.Sprintf("%v", conv.Labels)
		}
		lastActivity := formatTimestamp(conv.LastActivityAt)

		rows = append(rows, []string{
			strconv.Itoa(conv.ID),
			conv.Status,
			sender,
			assignee,
			inboxName,
			labels,
			lastActivity,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

type ConversationViewCmd struct {
	ID int `arg:"" help:"Conversation ID."`
}

func (c *ConversationViewCmd) Run(app *App) error {
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
	assignee := ""
	if conv.Meta.Assignee != nil {
		assignee = conv.Meta.Assignee.Name
	}
	team := ""
	if conv.Meta.Team != nil {
		team = conv.Meta.Team.Name
	}
	labels := ""
	if len(conv.Labels) > 0 {
		labels = fmt.Sprintf("%v", conv.Labels)
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
		{Key: "Assignee", Value: assignee},
		{Key: "Team", Value: team},
		{Key: "Channel", Value: conv.Meta.Channel},
		{Key: "Labels", Value: labels},
		{Key: "Messages", Value: strconv.Itoa(conv.MessagesCount)},
		{Key: "Created", Value: formatTimestamp(conv.CreatedAt)},
		{Key: "Last Activity", Value: formatTimestamp(conv.LastActivityAt)},
	})

	return nil
}

func formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}
