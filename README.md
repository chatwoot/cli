# chatwoot-cli

Read-only CLI for interacting with a [Chatwoot](https://www.chatwoot.com) instance from the terminal.

![chatwoot-cli](.github/chatwoot-cli.webp)

## Install

```bash
go install github.com/chatwoot/chatwoot-cli/cmd/chatwoot@latest
```

Or build from source:

```bash
git clone https://github.com/chatwoot/chatwoot-cli.git
cd chatwoot-cli
go build -o chatwoot ./cmd/chatwoot/
```

## Setup

```bash
chatwoot auth login
```

You'll be prompted for:
- **Base URL** — your Chatwoot instance (e.g. `https://app.chatwoot.com`)
- **API Key** — your agent API access token
- **Account ID** — your account number

Credentials are validated against the API before saving. Config is stored at `~/.chatwoot/config.yaml`.

## Usage

```
chatwoot <command> [flags]
```

### Conversations

```bash
chatwoot conversation list                     # List open conversations assigned to you
chatwoot conv list -s resolved                 # List resolved conversations
chatwoot conv list --assignee all --inbox 5    # All conversations in inbox 5
chatwoot conv list -l billing,urgent           # Filter by labels
chatwoot conversation view 42                  # View conversation details
```

### Messages

```bash
chatwoot message list 42                       # Messages in conversation #42
chatwoot msg list 42 --before 1000             # Messages before ID 1000
```

### Contacts

```bash
chatwoot contact list                          # List contacts
chatwoot contact view 123                      # View contact details
chatwoot contact search "john"                 # Search by name, email, or phone
```

### Inboxes

```bash
chatwoot inbox list                            # List all inboxes
chatwoot inbox view 5                          # View inbox details
```

### Agents

```bash
chatwoot agent list                            # List all agents
```

### Profile

```bash
chatwoot profile                               # Show your profile
```

### Auth & Config

```bash
chatwoot auth login                            # Interactive login
chatwoot auth logout                           # Remove saved credentials
chatwoot auth status                           # Show current user and instance
chatwoot config path                           # Print config file path
chatwoot config view                           # Print config (API key masked)
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output format: `text`, `json`, `csv` |
| `--account` | `-a` | Override account ID |
| `--quiet` | `-q` | Print only IDs (for scripting) |
| `--no-color` | | Disable colored output |
| `--verbose` | `-v` | Show request/response details |
| `--version` | | Print version |

## Output Formats

**Text** (default) — human-readable tables:

```
ID   Status  Contact       Assignee       Inbox
194  open    Jane Doe      Shivam Mishra  WebWidget
197  open    Vinay K       Shivam Mishra  Whatsapp
```

**JSON** — full API response, pipe to `jq`:

```bash
chatwoot conversation list -o json | jq '.[].id'
```

**CSV** — for spreadsheets and data processing:

```bash
chatwoot agent list -o csv > agents.csv
```

**Quiet** — IDs only, one per line:

```bash
chatwoot conversation list -q | xargs -I{} chatwoot conversation view {}
```

## License

MIT
