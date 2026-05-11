# Chatwoot CLI

CLI for [Chatwoot](https://www.chatwoot.com) — manage conversations, send replies, and run common workflows from your terminal.

## Install

**macOS / Linux** — install script (detects OS/arch, fetches the matching release binary, verifies SHA256):

```bash
curl -fsSL https://chwt.app/install-cli | sh
```

For a specific version or Windows, see the [install docs](https://developers.chatwoot.com/cli#install).

## Setup

```bash
chatwoot auth login
```

You'll be prompted for your **Base URL**, **API Key**, and **Account ID**. Credentials are validated before saving. Non-secret config lives at `~/.chatwoot/config.yaml`; the API key is stored in your OS keyring. For CI or headless environments, set `CHATWOOT_API_KEY` to override the keyring.

## Agent Skill

If you use Claude Code, Cursor, or another AI coding assistant, install the agent skill so it knows the CLI's grammar and safety rules before sending customer-visible replies:

```bash
npx skills add chatwoot/cli              # current project
npx skills add chatwoot/cli --global     # all projects
```

See the [agent skill docs](https://developers.chatwoot.com/cli/agent-skill) for details.

## Usage

The CLI uses a simple noun grammar:

- **Plural noun = list:** `chatwoot convs`, `chatwoot contacts`, `chatwoot agents`
- **`<noun> <id>`** views: `chatwoot conv 123`
- **`<noun> <id> <verb>`** acts: `chatwoot conv 123 reply "thanks"` — id before verb, the way you'd say it.

```bash
chatwoot convs                                 # Open conversations assigned to you
chatwoot convs --assignee all --inbox 5        # All conversations in inbox 5
chatwoot convs --query "refund"                # Search by message content

chatwoot conv 123                              # View
chatwoot conv 123 reply "Looking into it"
chatwoot conv 123 reply "internal note" --private
chatwoot conv 123 resolve                      # Or: open, pending, snooze
chatwoot conv 123 assign --agent me            # Or: --agent alice, --agent 42, --team 7
chatwoot conv 123 label billing,urgent
chatwoot conv 123 priority urgent              # urgent | high | medium | low | none

chatwoot contacts --search "john"
chatwoot contact 456 conversations

chatwoot inboxes / agents / labels / teams     # List
chatwoot me                                    # Your profile
```

Run `chatwoot --help` or see the [full command reference](https://developers.chatwoot.com/cli/commands).

## Output Formats

`-o text` (default), `-o json`, `-o csv`, or `-q` (IDs only, for scripting):

```bash
chatwoot convs -o json | jq '.data.payload[].id'
chatwoot convs -q | xargs -I{} chatwoot conv view {}
```

For piping, batching, and CI workflows, see the [scripting guide](https://developers.chatwoot.com/cli/scripting).

## License

MIT
