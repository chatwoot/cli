---
name: chatwoot-cli
description: >
  Operate Chatwoot helpdesks from the terminal — list and triage conversations,
  send replies and private notes, assign agents and teams, change status, set
  labels and priority, search contacts, and inspect inboxes via the `chatwoot`
  CLI. Use when the user wants to read, summarize, or act on Chatwoot
  conversations from the shell, scripts, agent workflows, or CI. Always load
  this skill before running `chatwoot` commands — it contains the noun/verb
  grammar, the output-format contract, and the safety rules that prevent
  customer-visible mistakes.
license: MIT
metadata:
  author: chatwoot
  homepage: https://www.chatwoot.com
  source: https://github.com/chatwoot/cli
---

# Chatwoot CLI

## Agent Protocol

The CLI defaults to human-readable text output. **It does NOT auto-switch to
JSON in non-TTY environments.** Agents must opt in to a parseable format
explicitly.

**Rules for agents:**
- Pass `-o json` whenever you need to parse output. Pipe to `jq` — never
  grep the text format.
- Pass `-q` (quiet) for ID-only output, one per line, suitable for piping to
  `xargs` or chaining `chatwoot` invocations.
- Default text output is for humans only; treat it as opaque.
- Exit `0` = success, non-zero = error. Errors go to stderr.
- Authenticate via the OS keyring (`chatwoot auth login`) for local use, or
  the `CHATWOOT_API_KEY` env var for CI / agent / headless contexts. Never
  rely on interactive prompts in scripts.
- `chatwoot auth login` is interactive (prompts for base URL, API key,
  account ID). If invoked headlessly it will fail — surface the env-var path
  instead.
- Prefer first-class commands over `chatwoot api`. Use raw API calls only when
  no command exists or the user explicitly asks for an endpoint-level call.
- Before raw API calls, check the application Swagger:
  https://raw.githubusercontent.com/chatwoot/chatwoot/develop/swagger/tag_groups/application_swagger.json
- Use `-v` (verbose) to see the underlying HTTP request/response when
  debugging an unexpected result.

## Grammar

The CLI reads the way you'd say it. **Memorize this — every command follows
one of three shapes:**

| Shape                              | Meaning              | Example                              |
|------------------------------------|----------------------|--------------------------------------|
| `<plural-noun>`                    | list                 | `chatwoot convs`, `chatwoot contacts`|
| `<singular-noun> <id>`             | view (shorthand)     | `chatwoot conv 123`                  |
| `<singular-noun> <id> <verb> [..]` | act on one resource  | `chatwoot conv 123 reply "hi"`       |

Nouns: `conv`/`convs`, `contact`/`contacts`, `inbox`/`inboxes`, `agents`,
`labels`, `teams`. The id always comes **before** the verb. See "Available
Commands" below for the full verb list.

When unsure, ask the CLI:
```bash
chatwoot --help
chatwoot conv --help          # all verbs on a conversation
chatwoot convs --help         # filters for the list command
```

## Global Flags

| Flag                | Description                                         |
|---------------------|-----------------------------------------------------|
| `-o, --output`      | Output format: `text` (default), `json`, `csv`      |
| `-a, --account`     | Override account ID for this invocation             |
| `-q, --quiet`       | Print only IDs, one per line — for scripting        |
| `--no-color`        | Disable colored output                              |
| `-v, --verbose`     | Show request/response details (debugging)           |
| `--version`         | Print CLI version                                   |

## Available Commands

| Command                          | What it does                                      |
|----------------------------------|---------------------------------------------------|
| `convs`                          | List conversations (filters: status, inbox, assignee, team, label, query, page) |
| `conv <id>`                      | View one conversation (shorthand for `view`)      |
| `conv <id> messages`             | List messages in a conversation                   |
| `conv <id> reply <text>`         | Send a public reply (use `--private` for a note)  |
| `conv <id> resolve`              | Mark resolved                                     |
| `conv <id> open`                 | Set status to open                                |
| `conv <id> pending`              | Set status to pending                             |
| `conv <id> snooze [--until X]`   | Snooze (default: until next reply)                |
| `conv <id> assign`               | Assign `--agent` and/or `--team`                  |
| `conv <id> unassign`             | Remove the assignee                               |
| `conv <id> label <a,b,c>`        | **Replace** labels with this set                  |
| `conv <id> priority <level>`     | `urgent`, `high`, `medium`, `low`, `none`         |
| `conv <id> contact`              | View the contact (sender) for the conversation    |
| `contacts`                       | List/search contacts                              |
| `contact <id>` / `<id> conversations` | View a contact / list their conversations    |
| `inboxes` / `inbox <id>`         | List inboxes / view one                           |
| `agents` / `labels` / `teams`    | List account-level resources                      |
| `me` / `whoami` / `auth status`  | Show current identity                             |
| `api <path>`                      | Call an arbitrary Chatwoot API endpoint with saved auth headers |
| `auth login` / `logout`          | Interactive login / remove credentials            |
| `config path` / `config view`    | Inspect config file location and contents         |
| `completion <shell>`             | Print shell-completion script                     |

## Common Mistakes

| # | Mistake | Fix |
|---|---------|-----|
| 1 | **Parsing default text output** | Text format is for humans and can change. Always pass `-o json` (or `-q` for IDs only) when an agent will consume the output. |
| 2 | **Forgetting `convs` defaults to your open queue** | `chatwoot convs` is implicitly `--assignee me -s open`. Pass `--assignee all` and the relevant `-s` (one of `open`, `pending`, `resolved`, `snoozed`) when you mean "everything". |
| 3 | **`label` is replace, not append** | `chatwoot conv 123 label foo` removes any existing labels other than `foo`. To add one label, fetch existing first: `chatwoot conv 123 -o json \| jq -r '.labels // [] \| join(",")'`, then pass the full set. See "Append a label" below — running the `label` command with an empty `$existing` (e.g. on fetch failure) silently strips every label, so use `set -o pipefail` and verify before re-setting. |
| 4 | **Confusing `--query` with contact search** | `convs --query` searches **message content**. To find a contact by name/email/phone, use `contacts --search`. |
| 5 | **Ambiguous `--agent <name>`** | `assign --agent <name>` matches a case-insensitive **substring** — risky when names overlap. Prefer agent IDs in scripts; run `chatwoot agents -o json` first to resolve. |
| 6 | **`-l a -l b` as repeated flags** | Labels are comma-separated on a single flag: `-l a,b`. Repeating the flag won't merge them. |
| 7 | **Assuming list = all** | List commands return one page. Inspect `meta` in `-o json` (and use `-p N` to advance) before assuming completeness. |
| 8 | **Snooze without `--until` is not "forever"** | Bare `snooze` snoozes until the customer's next reply, not indefinitely. Pass `--until 7d` or an absolute date for a fixed window. |
| 9 | **Running `auth login` in a script** | Interactive only — fails in non-TTY contexts. Use `CHATWOOT_API_KEY` plus the saved `~/.chatwoot/config.yaml` (or `-a` to override account). |

## Safety — customer-visible writes

Some commands change shared state or send messages a customer or teammate
will see. Treat all write operations as privileged actions. Before running any
of them in an agent context, **show the user the exact command and get explicit
approval. Never perform writes without user confirmation.** Don't assume
approval on one conversation extends to another.

Customer- or team-visible (effectively irreversible):
- `reply` (without `--private`) — the message is sent and cannot be unsent.
  Show the full reply text and confirm tone before sending.
- `assign` / `unassign` — appears in queues, may trigger notifications.
- `resolve` / `open` / `pending` / `snooze` — visible status changes; may
  close out SLA tracking.
- `label` — overwrites the existing label set (see mistake #3).
- `priority` — visible in dashboards, used for SLA routing.
- `api -X <method> ...` or `api --data ...` — arbitrary endpoint calls can
  mutate any supported resource. Treat non-GET requests as writes unless the
  endpoint contract proves otherwise. Show the exact method, path, and body
  before running a mutating raw API call.
- Any bulk operation composed with `-q | xargs` — pause, list what would be
  affected, then confirm.

Read-only and safe to run freely:
`convs`, `conv <id>` (view), `conv <id> messages`, `conv <id> contact`, `contacts`, `contact <id>`,
`inboxes`, `inbox <id>`, `agents`, `labels`, `teams`, `me`, `whoami`,
`auth status`, `config path`, `config view`, `api <path>` when it is a GET.

## Common Patterns

These show non-obvious composition (jq paths, label-append, bulk via `-q`).
For straight verb usage, the Available Commands table is canonical.

**List conversations** — list responses are wrapped in `.data.payload[]`:
```bash
chatwoot convs --assignee me -s open -o json \
  | jq '.data.payload[] | {id, contact: .meta.sender.name, last: .messages[-1].content}'

chatwoot convs --query "refund" --assignee all -s open -q   # IDs only
```

**Read recent messages** — `message_type`: `0` customer, `1` agent, `2` activity, `3` template:
```bash
chatwoot conv 123 messages -o json \
  | jq '.payload[-5:][] | {dir: (if .message_type==0 then "in" else "out" end), private, content}'
```

**Append a label** (`label` replaces — fetch first, then merge). `set -o pipefail` is required so a failed fetch surfaces instead of silently producing an empty `$existing`, which would clear every label on the next line:
```bash
set -o pipefail
existing=$(chatwoot conv 123 -o json | jq -r '.labels // [] | join(",")')
chatwoot conv 123 label "${existing:+$existing,}billing"
```

**Bulk via `-q | xargs`:**
```bash
chatwoot convs -l spam -q | xargs -I{} chatwoot conv {} resolve
```

**Chain contact → conversations:**
```bash
id=$(chatwoot contacts --search "jane@example.com" -o json | jq '.payload[0].id')
chatwoot contact "$id" conversations -o json
```

**Raw API call** — account-relative paths are expanded under `/api/v1/accounts/<account_id>`, so do not include the `/api/v1/accounts/...` prefix:
```bash
chatwoot api /conversations/123 -o json
chatwoot api -X PATCH /conversations/123 --data '{"status":"open"}'
```

Use the application Swagger as the endpoint reference before raw API calls:
https://raw.githubusercontent.com/chatwoot/chatwoot/develop/swagger/tag_groups/application_swagger.json
