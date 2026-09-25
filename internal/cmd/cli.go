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
	Output  string `short:"o" default:"text" enum:"text,json,csv" help:"How to print results: text (for people), json, or csv."`
	Account string `short:"a" env:"CHATWOOT_ACCOUNT" placeholder:"NAME" help:"Which account to use: a name from 'chatwoot accounts', or an account ID. Same as putting @name first."`
	Quiet   bool   `short:"q" help:"Print only IDs, one per line. Handy in scripts."`
	NoColor bool   `help:"Turn off colors."`
	Verbose bool   `short:"v" help:"Show each HTTP request and response (for debugging)."`

	// Plurals list; singulars view or act on one item.
	Convs       ConvsCmd    `cmd:"" aliases:"conversations" group:"Conversations" help:"List conversations."`
	Conv        ConvCmd     `cmd:"" aliases:"conversation" group:"Conversations" help:"View or act on one conversation: reply, resolve, assign, label, and more."`
	Contacts    ContactsCmd `cmd:"" group:"Contacts and team" help:"List or search contacts."`
	Contact     ContactCmd  `cmd:"" group:"Contacts and team" help:"View a contact or their conversations."`
	Inboxes     InboxesCmd  `cmd:"" group:"Contacts and team" help:"List inboxes."`
	Inbox       InboxCmd    `cmd:"" group:"Contacts and team" help:"View an inbox."`
	Agents      AgentsCmd   `cmd:"" group:"Contacts and team" help:"List agents."`
	Teams       TeamsCmd    `cmd:"" group:"Contacts and team" help:"List teams."`
	Labels      LabelsCmd   `cmd:"" group:"Contacts and team" help:"List the labels you can put on conversations."`
	HelpCenters HCsCmd      `cmd:"" name:"hcs" aliases:"help-centers" group:"Help center" help:"List help centers."`
	HelpCenter  HCCmd       `cmd:"" name:"hc" aliases:"help-center" group:"Help center" help:"Search and read help center articles."`

	// Setup.
	Auth     AuthCmd     `cmd:"" group:"Accounts and login" help:"Log in, log out, or see who you're logged in as."`
	Accounts AccountsCmd `cmd:"" group:"Accounts and login" help:"See your accounts, pick up new ones, or rename them."`
	Use      UseCmd      `cmd:"" group:"Accounts and login" help:"Choose your default account."`
	Me       MeCmd       `cmd:"" group:"Accounts and login" help:"Show who you're logged in as, and where (same as 'auth status')."`
	Whoami   WhoamiCmd   `cmd:"" group:"Accounts and login" help:"Show who you're logged in as, and where (same as 'auth status')."`

	Api        ApiCmd                    `cmd:"" group:"Other" help:"Call any Chatwoot API endpoint, for things without a command yet."`
	Config     ConfigCmd                 `cmd:"" aliases:"cfg" group:"Other" help:"Show your saved settings and where they live."`
	Completion kongcompletion.Completion `cmd:"" group:"Other" help:"Set up tab completion for your shell."`
	Version    VersionCmd                `cmd:"" group:"Other" help:"Show the CLI version."`

	VersionFlag kong.VersionFlag `name:"version" help:"Show the CLI version."`
}

// Help is shown under the one-line description in `chatwoot --help`.
func (c *CLI) Help() string {
	return `Commands read the way you'd say them: which account, what, which one, then
what to do. Leave out the account to use your default.

Examples:
  chatwoot convs                      Your open conversations
  chatwoot conv 123                   Look at conversation 123
  chatwoot conv 123 reply "On it!"    Reply to it
  chatwoot @acme convs                The same list, in your acme account

Copied a link from the dashboard? Use it in place of "conv 123":
  chatwoot https://app.chatwoot.com/app/accounts/1/conversations/123 resolve

New here? Start with: chatwoot auth login`
}
