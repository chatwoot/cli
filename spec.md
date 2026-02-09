# Chatwoot CLI Specification

Read-only CLI for interacting with a Chatwoot instance from the terminal.

## Authentication & Configuration

Config lives at `~/.chatwoot/config.yaml`:

```yaml
base_url: https://app.chatwoot.com
api_key: your_api_access_token
account_id: 1
```

- `chatwoot auth login` — interactive prompt for base URL, API key, and account ID. Validates credentials with a test API call before saving.
- `chatwoot auth logout` — deletes the config file.
- `chatwoot auth status` — prints the current authenticated user and instance URL.
- `chatwoot config path` — prints the config file path.
- `chatwoot config view` — prints the current config (API key masked).

Any command run without a valid config prompts the user to run `chatwoot auth login`.

## Global Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--account` | `-a` | int | from config | Override account ID |
| `--output` | `-o` | string | `text` | Output format: `text`, `json`, `csv` |
| `--quiet` | `-q` | bool | false | Print only IDs |
| `--no-color` | | bool | false | Disable colored output |
| `--verbose` | `-v` | bool | false | Print HTTP request/response details |
| `--help` | `-h` | bool | | Show help |
| `--version` | | bool | | Print CLI version |

## Output Formats

- **text** — human-readable table/list. Default for interactive terminals.
- **json** — raw JSON from the API response. Intended for piping to `jq`.
- **csv** — comma-separated values with a header row.

When stdout is not a TTY and no `--output` is specified, default to `json`.

## Commands

### `chatwoot conversation list`

List conversations with filters.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--status` | `-s` | string | `open` | `open`, `resolved`, `pending`, `snoozed` |
| `--inbox` | `-i` | int | | Filter by inbox ID |
| `--assignee` | | string | `me` | `me`, `unassigned`, `all` |
| `--team` | | int | | Filter by team ID |
| `--label` | `-l` | strings | | Comma-separated labels |
| `--sort` | | string | `latest` | `latest`, `created_at`, `priority` |
| `--page` | `-p` | int | 1 | Page number |
| `--limit` | `-n` | int | 25 | Results per page |

Text output columns: `ID`, `Status`, `Contact`, `Assignee`, `Inbox`, `Labels`, `Last Activity`.

### `chatwoot conversation view <id>`

Display a single conversation's details and its recent messages.

Shows: conversation metadata (status, inbox, assignee, team, labels, contact info) followed by the most recent messages.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--messages` | `-n` | int | 10 | Number of recent messages to show |

### `chatwoot message list <conversation-id>`

List messages in a conversation.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--before` | | int | | Messages before this message ID |
| `--after` | | int | | Messages after this message ID |
| `--limit` | `-n` | int | 20 | Number of messages |

Text output: sender name, timestamp, message content. Private notes are visually distinguished.

### `chatwoot contact list`

List contacts.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--page` | `-p` | int | 1 | Page number |
| `--limit` | `-n` | int | 25 | Results per page |

Text output columns: `ID`, `Name`, `Email`, `Phone`.

### `chatwoot contact view <id>`

Display a single contact's details: name, email, phone, company, conversations count, last seen.

### `chatwoot contact search <query>`

Search contacts by name, email, or phone.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--page` | `-p` | int | 1 | Page number |
| `--limit` | `-n` | int | 25 | Results per page |

### `chatwoot inbox list`

List all inboxes. Text output columns: `ID`, `Name`, `Channel Type`.

### `chatwoot team list`

List all teams. Text output columns: `ID`, `Name`, `Description`.

### `chatwoot agent list`

List all agents. Text output columns: `ID`, `Name`, `Email`, `Availability`.

### `chatwoot label list`

List all labels. Text output columns: `ID`, `Title`, `Description`, `Color`.

### `chatwoot canned list`

List canned responses. Text output columns: `ID`, `Short Code`, `Content`.

### `chatwoot report conversations`

View conversation report metrics.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--period` | | string | `weekly` | `daily`, `weekly`, `monthly` |
| `--since` | | string | | Start date (YYYY-MM-DD) |
| `--until` | | string | | End date (YYYY-MM-DD) |

### `chatwoot notification list`

List your notifications.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--page` | `-p` | int | 1 | Page number |

### `chatwoot profile`

Display the authenticated agent's profile (name, email, role, availability status).

## SDK Coverage

The existing SDK (`internal/sdk/`) covers:
- **Conversations**: list, get — ready
- **Messages**: list — ready
- **Labels**: list (per conversation) — ready

New SDK methods needed:
- **Contacts**: list, get, search
- **Inboxes**: list
- **Teams**: list
- **Agents**: list
- **Labels**: list (account-level)
- **Canned Responses**: list
- **Reports**: conversation metrics
- **Notifications**: list
- **Profile**: get current user

All SDK methods follow the existing pattern: service structs on `Client`, returning typed responses from `Client.Get()`.

## CLI Framework

Use [cobra](https://github.com/spf13/cobra) for command parsing and help generation.

Command tree:

```
chatwoot
├── conversation
│   ├── list
│   └── view
├── message
│   └── list
├── contact
│   ├── list
│   ├── view
│   └── search
├── inbox
│   └── list
├── team
│   └── list
├── agent
│   └── list
├── label
│   └── list
├── canned
│   └── list
├── report
│   └── conversations
├── notification
│   └── list
├── profile
├── auth
│   ├── login
│   ├── logout
│   └── status
└── config
    ├── path
    └── view
```

## Project Structure

```
cmd/
  chatwoot/
    main.go              # cobra root command, global flags
internal/
  cmd/
    root.go              # root command setup, global flag binding
    conversation.go      # conversation list, view
    message.go           # message list
    contact.go           # contact list, view, search
    inbox.go             # inbox list
    team.go              # team list
    agent.go             # agent list
    label.go             # label list
    canned.go            # canned list
    report.go            # report conversations
    notification.go      # notification list
    profile.go           # profile
    auth.go              # auth login, logout, status
    config.go            # config path, view
  config/
    config.go            # unchanged
  sdk/
    client.go            # unchanged
    conversations.go     # unchanged
    messages.go          # unchanged
    labels.go            # unchanged + account-level list
    contacts.go          # new
    inboxes.go           # new
    teams.go             # new
    agents.go            # new
    canned.go            # new
    reports.go           # new
    notifications.go     # new
    profile.go           # new
  output/
    output.go            # text/json/csv formatting dispatch
    table.go             # table renderer for text output
    json.go              # json output
    csv.go               # csv output
```

## Error Handling

- Missing/invalid config: print message directing to `chatwoot auth login`, exit 1.
- API errors: print the HTTP status and error body from Chatwoot, exit 1.
- Network errors: print a short message with the underlying error, exit 1.
- Invalid flags/arguments: cobra's built-in usage error, exit 2.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (API, network, config) |
| 2 | Usage error (bad flags, missing args) |
