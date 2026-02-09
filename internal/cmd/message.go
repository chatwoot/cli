package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

type MessageCmd struct {
	List MessageListCmd `cmd:"" help:"List messages in a conversation."`
}

type MessageListCmd struct {
	ConversationID int `arg:"" help:"Conversation ID."`
	Before         int `help:"Messages before this message ID."`
}

func (c *MessageListCmd) Run(app *App) error {
	resp, err := app.Client.Messages(c.ConversationID).List(c.Before)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	messages := resp.Payload
	if len(messages) == 0 {
		fmt.Println("No messages found.")
		return nil
	}

	headers := []string{"ID", "Type", "Sender", "Content", "Time"}
	rows := make([][]string, 0, len(messages))
	for _, msg := range messages {
		sender := ""
		if msg.Sender != nil {
			sender = msg.Sender.Name
		}
		msgType := messageTypeName(msg.MessageType)
		if msg.Private {
			msgType = "note"
		}
		content := truncate(strings.ReplaceAll(msg.Content, "\n", " "), 60)
		ts := formatTimestamp(msg.CreatedAt)

		rows = append(rows, []string{
			strconv.Itoa(msg.ID),
			msgType,
			sender,
			content,
			ts,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
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
