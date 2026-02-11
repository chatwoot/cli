# chatwoot-cli

Read-only CLI and interactive TUI for the Chatwoot API.

## Build & Run

```bash
go build ./cmd/chatwoot/        # build binary
mise run dev                     # auto-rebuild on file changes
./chatwoot                       # launch interactive TUI (requires auth)
./chatwoot conversation list     # CLI mode
```

## Project Structure

```
cmd/chatwoot/main.go       Entry point: no args → TUI, otherwise Kong CLI
internal/
  sdk/                     HTTP client + service modules (conversations, messages, contacts, etc.)
  cmd/                     Kong command structs with Run(app *App) error
  config/                  YAML config at ~/.chatwoot/config.yaml
  output/                  Printer: text (tabwriter), JSON, CSV formats + quiet mode
  tui/                     Bubbletea interactive TUI
```

## Architecture

- **CLI framework**: Kong (alecthomas/kong) — struct-based command tree with tags
- **TUI framework**: Bubbletea + Lipgloss + Bubbles
- **SDK pattern**: `client.Conversations()`, `client.Contacts()`, etc. return service objects
- **Command pattern**: each command is a struct with `Run(app *App) error`
- **App struct** holds `Client`, `Printer`, `Config` — passed to all commands

## Key Conventions

- Kong commands: define flags/args as struct fields with tags, implement `Run(app *App) error`
- `skipAuth` in main.go: auth/config commands bypass API client creation
- `GetRaw()` on Client: for non-account-scoped endpoints (e.g. `/api/v1/profile`)
- TUI layout: lipgloss `Width(w)` sets content width; borders add +2 visual cols — always account for this

## Chatwoot API Quirks

- Contacts list `meta.current_page` returns as string, not int
- Messages list `meta.agent_last_seen_at` can be string
- Single contact GET returns `{payload: {contact data}}` (wrapped)
- Agents list returns raw `[]Agent` array (not wrapped in payload)
- Profile endpoint is non-account-scoped: `/api/v1/profile`

## Commits

Use conventional commits without scope: `feat:`, `fix:`, `chore:`, `refactor:`, `docs:`
