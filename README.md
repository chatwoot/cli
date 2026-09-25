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
chatwoot auth login                            # asks for the URL (default: Chatwoot Cloud)
chatwoot auth login staging.chatwoot.com       # or pass it; any dashboard link works too
```

You'll be prompted for your **access token**. The CLI then registers every account you belong to on that instance, named after the account (`acme`, `globex-inc`), and asks which one is the default. Non-secret config lives at `~/.chatwoot/config.yaml`; tokens are stored in your OS keyring. For CI or headless environments, set `CHATWOOT_API_KEY` to override the keyring.

## Multiple accounts

Log in once per Chatwoot instance; every account you can see there is registered.

```bash
chatwoot accounts                              # List accounts (* = default)
chatwoot @acme convs                           # Use an account for one command (unique prefixes work: @ac)
chatwoot -a acme convs                         # Same, as a flag
chatwoot use acme                              # Change the saved default
CHATWOOT_ACCOUNT=acme chatwoot convs           # Per shell session or CI

chatwoot https://app.chatwoot.com/app/accounts/1/conversations/4521              # Paste a link
chatwoot https://app.chatwoot.com/app/accounts/1/conversations/4521 reply "hi"   # ...and act on it

chatwoot accounts --refresh                    # Pick up accounts added or removed since login
chatwoot accounts rename chatwoot-staging stg  # Your own names
chatwoot auth logout staging.chatwoot.com      # Log out of one instance
```

The account is chosen in this order: a pasted link, then `@name` / `-a`, then `CHATWOOT_ACCOUNT`, then the default. When the same account name exists on two instances, the second gets the host added (`chatwoot-staging`). Writes to a non-default account print `→ <name>` to stderr first. Upgrading from a single-account version needs no action.

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

chatwoot hcs                                   # List help centers
chatwoot hc default chatwoot-support       # Save default help center and locale
chatwoot hc articles --query "account"
chatwoot hc articles --category getting-started
chatwoot hc article create-a-agent-bot
chatwoot hc articles --portal other-help-center --locale fr

chatwoot api /conversations/123                # Expands to /api/v1/accounts/<id>/conversations/123
chatwoot api -X PATCH /conversations/123 --data '{"status":"open"}'
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
