# internal/cmd - Kong CLI Commands

Kong command definitions for CLI mode (non-TUI). Each command is a struct with fields representing flags/args, implementing `Run(app *App) error` to execute the command logic.

## Structure

**Kong Pattern:**
```go
type MyCommand struct {
    Flag1 string `help:"description" default:"value"`
    Arg1  string `arg:"" help:"positional argument"`
}

func (c *MyCommand) Run(app *App) error {
    // Use app.Client, app.Printer, app.Config
}
```

## Command Files

### root.go
Root CLI command and help. Provides:
- Version and help display
- Subcommand dispatcher (auth, config, conversation, message, contact, etc.)
- Global flags (verbose, format)
- Help text with command summary

### auth.go
Authentication setup.
- `auth login` — save API token and base URL to config
- `auth logout` — clear credentials
- Stores credentials in `~/.chatwoot/config.yaml`

### config.go
Configuration management.
- `config show` — display current config
- `config reset` — clear all settings

### conversation.go
Conversation queries.
- `conversation list` — list conversations (filters: assignee_type, status, page)
- `conversation get <ID>` — single conversation details
- Output: table, JSON, or CSV via Printer

### message.go
Message queries.
- `message list <CONV_ID>` — messages in conversation (pagination via beforeID)
- `message get <ID>` — single message details
- Output: formatted text or JSON

### contact.go
Contact queries.
- `contact list` — all contacts (paginated)
- `contact get <ID>` — full contact details with additional attributes
- Output: table with name, email, phone, or JSON for details

### label.go
Label queries.
- `label list` — all labels/tags for account
- Output: table or JSON

### agent.go
Agent (team member) queries.
- `agent list` — all agents with availability status
- Output: table or JSON

### inbox.go
Inbox configuration.
- `inbox list` — all inboxes with channel types
- Output: table or JSON

### profile.go
Authenticated user profile.
- `profile show` — current user details
- Output: formatted text or JSON

## App Struct

Passed to every command:
```go
type App struct {
    Client   *sdk.Client
    Printer  *output.Printer
    Config   *config.Config
    Version  string
}
```

Commands use:
- `app.Client` for API calls
- `app.Printer.Print()` to render output (respects --format flag)
- `app.Config` for cached values (base URL, account ID, etc.)

## Output Formatting

Printer supports three formats:
- **Text** (default): tabwriter-formatted tables
- **JSON**: structured JSON objects
- **CSV**: comma-separated values

Set via `--format json` or `--format csv` on any command.

## CLI Usage Examples

```bash
# List conversations
./chatwoot conversation list --assignee-type me --status open

# Get single conversation
./chatwoot conversation get 123

# List messages in conversation
./chatwoot message list 123

# Get contact details
./chatwoot contact get 456

# Show authenticated user
./chatwoot profile show --format json
```

## skipAuth Flag

In `main.go`, `auth` and `config` commands are marked with `skipAuth: true` — they don't require an API client. Other commands fail gracefully if auth is not configured.

## TODO

- Implement `conversation create` for ticket creation
- Add bulk export (all conversations to CSV)
- Implement `message send` via CLI (currently reply editor is TUI-only)
- Add filtering by label, agent, date range
