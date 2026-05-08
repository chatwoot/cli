# chatwoot-cli

CLI for [Chatwoot](https://www.chatwoot.com) — manage conversations, send replies, and run common workflows from your terminal.

## Install

**macOS / Linux** — install script (detects OS/arch, fetches the matching release binary, verifies SHA256):

```bash
curl -fsSL https://chwt.app/install-cli | sh
```

The script installs to `~/.local/bin/chatwoot` by default. Override with environment variables:

```bash
CHATWOOT_VERSION=v0.2.0 CHATWOOT_INSTALL_DIR=/usr/local/bin curl -fsSL https://chwt.app/install-cli | sh
```

**Windows** — download `chatwoot_<version>_Windows_x86_64.zip` from the [releases page](https://github.com/chatwoot/cli/releases/latest) and extract `chatwoot.exe`.

**From source** — requires Go 1.25+:

```bash
go install github.com/chatwoot/cli/cmd/chatwoot@latest
# or
git clone https://github.com/chatwoot/cli.git && cd cli
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

Credentials are validated against the API before saving. Non-secret config is stored at `~/.chatwoot/config.yaml`; the API key is stored in your OS keyring. For CI, coding agents, or headless environments, set `CHATWOOT_API_KEY` to override the saved keyring token.

## Agent Skill

If you use Claude Code, Cursor, or another AI coding assistant, install the agent skill so it knows the CLI's grammar, JSON shapes, and safety rules before sending customer-visible replies:

```bash
npx skills add chatwoot/cli              # current project
npx skills add chatwoot/cli --global     # all projects
```

Agents run non-interactively, so authenticate once with `chatwoot auth login` (or set `CHATWOOT_API_KEY` for sandboxed environments). See the [agent skill docs](https://www.chatwoot.com/docs/cli/agent-skill) for details.

## Shell Completions

The install script offers to set this up interactively. To do it manually, write the completion code to your shell's standard auto-load location:

```bash
# bash
chatwoot completion bash -c > ~/.local/share/bash-completion/completions/chatwoot

# fish
chatwoot completion fish -c > ~/.config/fish/completions/chatwoot.fish

# zsh — append a source line to your .zshrc
echo 'source <(chatwoot completion zsh -c)' >> ~/.zshrc
```

Restart your shell (or `source` the rc file) and tab-completion will work for commands, subcommands, and flags. Run `chatwoot completion --help` for details.

## CLI Usage

The CLI uses a simple noun grammar:

- **Plural noun = list:** `chatwoot convs`, `chatwoot contacts`, `chatwoot agents`
- **Singular + id + verb:** `chatwoot conv 123 reply "thanks"` — id before verb, the way you'd say it.
- **`<noun> <id>`** alone is shorthand for view: `chatwoot conv 123` views conversation 123.

### Conversations — list

```bash
chatwoot convs                                 # Open conversations assigned to you (default)
chatwoot convs -s resolved                     # Resolved conversations
chatwoot convs --assignee all --inbox 5        # All conversations in inbox 5
chatwoot convs -l billing,urgent               # Filter by labels
chatwoot convs --query "refund"                # Search by message content
```

### Conversations — act on one

```bash
chatwoot conv 123                              # View (default)
chatwoot conv 123 messages                     # List messages
chatwoot conv 123 reply "Thanks, looking into it"
chatwoot conv 123 reply "internal note" --private
chatwoot conv 123 resolve                      # Mark resolved
chatwoot conv 123 open                         # Set status to open
chatwoot conv 123 pending                      # Set status to pending
chatwoot conv 123 snooze                       # Snooze until next reply
chatwoot conv 123 snooze --until 24h           # Or 7d, 2026-05-10
chatwoot conv 123 assign --agent me            # Assign to yourself
chatwoot conv 123 assign --agent alice         # By name (case-insensitive substring)
chatwoot conv 123 assign --agent 42            # By agent ID
chatwoot conv 123 assign --team 7              # Assign to a team
chatwoot conv 123 unassign
chatwoot conv 123 label billing,urgent         # Set labels (replaces existing)
chatwoot conv 123 priority urgent              # urgent | high | medium | low | none
```

### Contacts

```bash
chatwoot contacts                              # List contacts
chatwoot contacts --search "john"              # Search by name, email, or phone
chatwoot contact 456                           # View
chatwoot contact 456 conversations             # Conversations for this contact
```

### Inboxes, agents, labels, teams

```bash
chatwoot inboxes                               # List inboxes
chatwoot inbox 5                               # View one
chatwoot agents                                # List agents
chatwoot labels                                # List account-level labels
chatwoot teams                                 # List teams
```

### Profile

```bash
chatwoot me                                    # Show your profile
```

### Auth & config

```bash
chatwoot auth login                            # Interactive login (caches user_id)
chatwoot auth logout                           # Remove saved credentials
chatwoot auth status                           # Show current user and instance
chatwoot config path                           # Print config file path
chatwoot config view                           # Print config and credential source
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
chatwoot convs -o json | jq '.data.payload[].id'
```

**CSV** — for spreadsheets and data processing:

```bash
chatwoot agents -o csv > agents.csv
```

**Quiet** — IDs only, one per line:

```bash
chatwoot convs -q | xargs -I{} chatwoot conv view {}
```

## License

MIT
