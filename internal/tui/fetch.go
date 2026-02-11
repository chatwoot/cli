package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
)

// Messages returned by async fetches

type conversationsMsg struct {
	conversations []sdk.Conversation
	err           error
}

type messagesMsg struct {
	conversationID int
	messages       []sdk.Message
	prepend        bool // true when loading older messages via pagination
	err            error
}

type contactMsg struct {
	contact *sdk.ContactFull
	err     error
}

type profileMsg struct {
	name string
	err  error
}

type replyMsg struct {
	conversationID int
	err            error
}

type toggleStatusMsg struct {
	conversationID int
	newStatus      string
	err            error
}

type tickMsg time.Time

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Commands

func fetchProfile(client *sdk.Client) tea.Cmd {
	return func() tea.Msg {
		profile, err := client.Profile().Get()
		if err != nil {
			return profileMsg{err: err}
		}
		return profileMsg{name: profile.Name}
	}
}

func fetchConversations(client *sdk.Client, status, assigneeType string, page int) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Conversations().List(sdk.ListOptions{
			Status:       status,
			AssigneeType: assigneeType,
			Page:         page,
			SortBy:       "last_activity_at_desc",
		})
		if err != nil {
			return conversationsMsg{err: err}
		}
		return conversationsMsg{conversations: resp.Data.Payload}
	}
}

// TODO: paginate messages using beforeID to load older messages on scroll
func fetchMessages(client *sdk.Client, convID int) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Messages(convID).List(0)
		if err != nil {
			return messagesMsg{conversationID: convID, err: err}
		}
		return messagesMsg{conversationID: convID, messages: resp.Payload, prepend: false}
	}
}

func fetchMoreMessages(client *sdk.Client, convID, beforeID int) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Messages(convID).List(beforeID)
		if err != nil {
			return messagesMsg{conversationID: convID, err: err, prepend: true}
		}
		return messagesMsg{conversationID: convID, messages: resp.Payload, prepend: true}
	}
}

func fetchContact(client *sdk.Client, contactID int) tea.Cmd {
	return func() tea.Msg {
		contact, err := client.Contacts().Get(contactID)
		if err != nil {
			return contactMsg{err: err}
		}
		return contactMsg{contact: contact}
	}
}

func toggleStatus(client *sdk.Client, convID int, status string, snoozedUntil *int64) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Conversations().ToggleStatus(convID, status, snoozedUntil)
		if err != nil {
			return toggleStatusMsg{conversationID: convID, err: err}
		}
		return toggleStatusMsg{conversationID: convID, newStatus: resp.CurrentStatus}
	}
}

func sendMessage(client *sdk.Client, convID int, content string, private bool) tea.Cmd {
	return func() tea.Msg {
		_, err := client.Messages(convID).Create(content, private)
		return replyMsg{conversationID: convID, err: err}
	}
}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
