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

func fetchMessages(client *sdk.Client, convID int) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Messages(convID).List(0)
		if err != nil {
			return messagesMsg{conversationID: convID, err: err}
		}
		return messagesMsg{conversationID: convID, messages: resp.Payload}
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

func sendMessage(client *sdk.Client, convID int, content string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.Messages(convID).Create(content, false)
		return replyMsg{conversationID: convID, err: err}
	}
}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
