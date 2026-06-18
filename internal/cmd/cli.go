package cmd

import (
	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
)

// CLI is the root Kong struct defining the entire command tree.
//
// Grammar:
//   - Plural noun = list:    `chatwoot convs`, `chatwoot contacts`
//   - Singular + verb + id:  `chatwoot conv view 123`, `chatwoot conv reply 123 "hi"`
//   - `conv 123` is shorthand for `conv view 123` (default subcommand).
type CLI struct {
	Output  string `short:"o" default:"text" enum:"text,json,csv" help:"Output format."`
	Account int    `short:"a" help:"Override account ID."`
	Quiet   bool   `short:"q" help:"Print only IDs."`
	NoColor bool   `help:"Disable colored output."`
	Verbose bool   `short:"v" help:"Show request/response details."`

	// Plurals — list commands.
	Convs       ConvsCmd    `cmd:"" aliases:"conversations" help:"List conversations."`
	Contacts    ContactsCmd `cmd:"" help:"List or search contacts."`
	Inboxes     InboxesCmd  `cmd:"" help:"List inboxes."`
	Agents      AgentsCmd   `cmd:"" help:"List agents."`
	Labels      LabelsCmd   `cmd:"" help:"List labels."`
	Teams       TeamsCmd    `cmd:"" help:"List teams."`
	HelpCenters HCsCmd      `cmd:"" name:"hcs" aliases:"help-centers" help:"List help centers."`

	// Global search across all buckets.
	Search SearchCmd `cmd:"" help:"Global search across conversations, contacts, messages, and help-center articles."`

	// Singulars — context for verbs and subresources.
	Conv       ConvCmd    `cmd:"" aliases:"conversation" help:"View or act on a conversation."`
	Contact    ContactCmd `cmd:"" help:"View a contact."`
	Inbox      InboxCmd   `cmd:"" help:"View an inbox."`
	HelpCenter HCCmd      `cmd:"" name:"hc" aliases:"help-center" help:"List, search, or view help center articles."`

	// Workflow. `me` and `whoami` are aliases of `auth status`.
	Me     MeCmd     `cmd:"" help:"Show identity and connection (alias of 'auth status')."`
	Whoami WhoamiCmd `cmd:"" help:"Show identity and connection (alias of 'auth status')."`
	Api    ApiCmd    `cmd:"" help:"Make an HTTP request to the Chatwoot API."`

	// Setup.
	Auth   AuthCmd   `cmd:"" help:"Login, logout, and status."`
	Config ConfigCmd `cmd:"" aliases:"cfg" help:"Manage CLI configuration."`

	Completion kongcompletion.Completion `cmd:"" help:"Print shell completion setup."`
	Version    VersionCmd                `cmd:"" help:"Print the CLI version."`

	VersionFlag kong.VersionFlag `name:"version" help:"Print the CLI version."`
}
