package cmd

import "github.com/alecthomas/kong"

// CLI is the root Kong struct defining the entire command tree.
type CLI struct {
	Output  string `short:"o" default:"text" enum:"text,json,csv" help:"Output format."`
	Account int    `short:"a" help:"Override account ID."`
	Quiet   bool   `short:"q" help:"Print only IDs."`
	NoColor bool   `help:"Disable colored output."`
	Verbose bool   `short:"v" help:"Show request/response details."`

	Conversation ConversationCmd `cmd:"" aliases:"conv" help:"List and view conversations."`
	Message      MessageCmd      `cmd:"" aliases:"msg" help:"View messages in a conversation."`
	Contact      ContactCmd      `cmd:"" help:"View and search contacts."`
	Inbox        InboxCmd        `cmd:"" help:"List and view inboxes."`
	Agent        AgentCmd        `cmd:"" help:"List agents."`
	Profile      ProfileCmd      `cmd:"" help:"Show your profile."`
	Auth         AuthCmd         `cmd:"" help:"Login, logout, and status."`
	Config       ConfigCmd       `cmd:"" aliases:"cfg" help:"Manage CLI configuration."`

	Version kong.VersionFlag `help:"Show version."`
}
