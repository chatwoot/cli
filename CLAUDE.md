# chatwoot-cli

CLI for the Chatwoot API.

## Build & Run

```bash
go build ./cmd/chatwoot/        # build binary
mise run dev                     # auto-rebuild on file changes
./chatwoot                       # no args → prints help
./chatwoot convs                 # plural noun = list
./chatwoot conv 123              # `conv 123` = view conv 123
./chatwoot conv 123 reply "hi"   # id-first verb dispatch
```

## Project Structure

```
cmd/chatwoot/main.go       Entry point: pre-parses id-first grammar, then Kong
internal/
  sdk/                     HTTP client + service modules (conversations, messages, contacts, etc.)
  cmd/                     Kong command structs with Run(app *App) error
  config/                  YAML config at ~/.chatwoot/config.yaml
  output/                  Printer: text (tabwriter), JSON, CSV formats + quiet mode
```

## Architecture

- **CLI framework**: Kong (alecthomas/kong) — struct-based command tree with tags
- **SDK pattern**: `client.Conversations()`, `client.Contacts()`, etc. return service objects
- **Command pattern**: each command is a struct with `Run(app *App) error`
- **App struct** holds `Client`, `Printer`, `Config` — passed to all commands

## Key Conventions

- Kong commands: define flags/args as struct fields with tags, implement `Run(app *App) error`
- Grammar: plural noun = list (`ConvsCmd`), singular noun = parent struct with verb subcommands; each verb's struct holds its own `arg:""` ID. Kong forbids mixing `arg:""` and `cmd:""` siblings, so internally the verb comes before the ID (`conv reply 123 "hi"`).
- `cmd/chatwoot/main.go` runs `rewriteIDFirstGrammar` to swap `<noun> <id> <verb>` → `<noun> <verb> <id>` before Kong parses, so users get to type the natural id-first form. A custom `kong.Help` printer flips the help output to match.
- `default:"withargs"` on the View subcommand routes `chatwoot conv 123` to `conv view 123`.
- `skipAuth` in main.go: auth/config commands bypass API client creation
- `GetRaw()` on Client: for non-account-scoped endpoints (e.g. `/api/v1/profile`)
- `Config.UserID` is cached on `auth login` and lazy-fetched on first `assign --agent me` so subsequent calls don't need a profile request

## Chatwoot API Quirks

- Contacts list `meta.current_page` returns as string, not int
- Messages list `meta.agent_last_seen_at` can be string
- Single contact GET returns `{payload: {contact data}}` (wrapped)
- Agents list returns raw `[]Agent` array (not wrapped in payload)
- Profile endpoint is non-account-scoped: `/api/v1/profile`

## Commits

Use conventional commits without scope: `feat:`, `fix:`, `chore:`, `refactor:`, `docs:`

## Releasing

Releases are tag-driven and the GitHub release body comes from `CHANGELOG.md`.

1. Move the relevant items from `## [Unreleased]` into a new `## [x.y.z] - YYYY-MM-DD` section in `CHANGELOG.md`, and add its `[x.y.z]: .../compare/...` link at the bottom. (Follow [Keep a Changelog](https://keepachangelog.com/) / SemVer.)
2. Commit, then push tag `vx.y.z` (must match the changelog version, minus the `v`).
3. The `release` workflow (`.github/workflows/release.yml`) runs tests, extracts that version's section from `CHANGELOG.md` with awk, and passes it to GoReleaser via `--release-notes`. **If no matching section exists, the release fails fast** — so the changelog must be updated before tagging.
4. GoReleaser (`.goreleaser.yml`) builds the cross-platform archives, checksums, and the GitHub release; its own `changelog:` block is only a fallback if `--release-notes` is ever dropped.

Dry run (local goreleaser must be v2; CI pins `2.15.4`):

```bash
go run github.com/goreleaser/goreleaser/v2@v2.15.4 check
go run github.com/goreleaser/goreleaser/v2@v2.15.4 release --snapshot --clean --skip=publish
```
