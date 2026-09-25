# internal/config - Configuration Management

YAML-based configuration persistence for non-secret account settings. Configuration is stored in `~/.chatwoot/config.yaml` and auto-loaded on startup. API keys are resolved from `CHATWOOT_API_KEY` first, then the OS keyring.

## Files

### config.go
Schema and file I/O.
- `Config` — `Version`, `Default` (an account name), `Accounts`
- `Account` — `Name`, `BaseURL`, `ID`, `UserID`, `UserName`, `AccountName`, per-account `HelpCenter`, `Provisional`
- `Load()` — returns nil when no file exists; converts a v1 (flat `base_url`/`account_id`) file **in memory** and flags it via `MigratedFromV1()`
- `Save()` — writes `version: 2` atomically (temp file + rename); the first time it replaces a v1 file, the original is kept as `<config>.bak`

### accounts.go
- `SyncAccounts(baseURL, userID, userName, memberships)` — reconciles one login's accounts with the profile's `accounts` list. Names never change once given, except provisional (migrated) ones, which get the real account name on the first sync. An empty list (older Chatwoot) removes nothing.
- Naming: slug of the account name; all-digit or empty names become `account-<id>`; a clash with another instance adds the host label (`chatwoot-staging`), a clash on the same instance adds the user's name (`acme-test-agent`).
- `Rename`, `SetDefault`, `RemoveBaseURL`, `FindByID`, `UserIDs`

### resolve.go
- `Resolve(selector)` — empty → default; digits → that account ID on the default's login (an unregistered ID returns an ad-hoc copy with `Name == ""`); a link → the account it names (`NotLoggedInError` for an unknown instance); otherwise exact name or unique prefix. Errors: `ErrNoDefaultAccount`, `ErrUnknownAccount`, `ErrAmbiguousAccount`.

### url.go
- `ParseInstanceURL` — bare host, base URL, or dashboard link → base URL (+ account ID when the link names one). A subpath before `/app/` is kept.
- `ParseLink` — dashboard link → `Link{BaseURL, AccountID, Noun, ID}` for conversation, contact, and inbox routes.

### credentials.go
Token resolution and OS keyring storage.
- `ResolveAPIKey(acct)` — `CHATWOOT_API_KEY` first; then the login entry `login:<base_url>#<user_id>`; then the pre-multi-account `api-key` entry (instance must match; copied to the login entry, never deleted); then the older `<base_url>/accounts/<id>` entry (migrated and removed).
- `SaveAPIKey(acct, key)` — requires `BaseURL` and `UserID`
- `DeleteAPIKeys()` — everything in this build's keyring service; `DeleteBaseURLAPIKeys(baseURL, userIDs)` — one instance

## Build Profiles (dev vs prod)

`configFileName` and `keyringService` are selected at build time via the `dev`
build tag (`profile_prod.go` for `//go:build !dev`, `profile_dev.go` for
`//go:build dev`):

| | config file | keyring service | `config.IsDev` |
|---|---|---|---|
| prod (default `go build`, releases) | `~/.chatwoot/config.yaml` | `chatwoot-cli` | `false` |
| dev (`go build -tags dev`, `mise run dev`) | `~/.chatwoot/config.dev.yaml` | `chatwoot-cli-dev` | `true` |

A dev build keeps its own credentials, so iterating on the CLI never reads or
clobbers the production login. The keyring **service** (not just the entry name)
is namespaced per profile, so `auth logout` — which does
`keyring.DeleteAll(keyringService)` to clear stale entries — only wipes the
active build's tokens and never the other profile's. Release builds (goreleaser
passes no tags) exclude `profile_dev.go` entirely — the dev path is compiled
out. `config view` shows a `Profile: dev` line on dev builds.

## Config Schema

```yaml
version: 2
default: acme
accounts:
  - name: acme
    base_url: https://app.chatwoot.com
    id: 42
    user_id: 7
    user_name: Shivam Mishra
    account_name: Acme
    help_center:
      default_portal_slug: acme-help
      default_locale: en
  - name: chatwoot-staging
    base_url: https://staging.chatwoot.com
    id: 1
    user_id: 3
```

Version 1 files (`base_url`, `account_id`, `user_id`, `help_center` at the top level) are still read and upgraded.

## File Permissions

Config directory is created with `0700`; config file is created with `0600`. API keys are not written to YAML.
