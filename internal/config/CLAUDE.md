# internal/config - Configuration Management

YAML-based configuration persistence for non-secret account settings. The
config document holds one or more **named profiles** (each a Chatwoot
instance + account) plus a default selection, stored in `~/.chatwoot/config.yaml`
and auto-loaded on startup. API keys are resolved from `CHATWOOT_API_KEY` first,
then the active profile's OS keyring entry.

## Files

### config.go
Profile-aware config struct and file I/O. Provides:
- `Config` — one profile's non-secret settings (BaseURL, AccountID, UserID, HelpCenter).
- `Store` — the on-disk document: `default_profile` + a `profiles` map.
- `LoadStore()` / `(*Store).Save()` — read/write the document; a legacy flat
  (pre-profiles) config is migrated into the `default` profile on load.
- `(*Store).ActiveName(override)` — resolve the active profile: override (flag)
  → `CHATWOOT_PROFILE` → `default_profile` → `"default"`.
- `(*Store).Get/Set/Remove/Names/IsEmpty` — profile management. `Remove` promotes
  another profile to default if the removed one was the default.
- `ResolveActiveName/LoadProfile/SaveProfile` — package-level helpers for callers
  that hold only a profile-override string (the command layer).
- `Load()` / `Save()` — compatibility shims that operate on the active profile.

### credentials.go
Per-profile credential resolution and OS keyring storage. Provides:
- `ResolveAPIKeyFor(profile, cfg)` — `CHATWOOT_API_KEY` first, then the profile's
  keyring entry. The `default` profile keeps the historical `api-key` entry (and
  migrates older URL/account-scoped entries forward), so pre-profiles logins
  resolve unchanged; named profiles use a `profile:<name>` entry.
- `SaveAPIKeyFor(profile, cfg, key)` / `DeleteAPIKeyFor(profile)` — per-profile.
- `ResolveAPIKey/SaveAPIKey` — shims for the active profile.
- `DeleteAPIKey` — wipes every entry under this build's keyring service (full reset).

## Build Profiles (dev vs prod) — distinct from named profiles

`configFileName` and `keyringService` are selected at build time via the `dev`
build tag (`profile_prod.go` for `//go:build !dev`, `profile_dev.go` for
`//go:build dev`):

| | config file | keyring service | `config.IsDev` |
|---|---|---|---|
| prod (default `go build`, releases) | `~/.chatwoot/config.yaml` | `chatwoot-cli` | `false` |
| dev (`go build -tags dev`, `mise run dev`) | `~/.chatwoot/config.dev.yaml` | `chatwoot-cli-dev` | `true` |

A dev build keeps its own credentials, so iterating on the CLI never reads or
clobbers the production login. The two concepts compose cleanly: the **build
profile** selects the config *file* and keyring *service*; **named profiles**
select an entry *within* them. `config view` shows a `Build: dev` line on dev
builds (the `Profile:` line now reports the active named profile).

## Config Schema

```yaml
default_profile: work
profiles:
  work:
    base_url: https://app.chatwoot.com
    account_id: 47
  staging:
    base_url: https://staging.chatwoot.com
    account_id: 3
```

## Usage

Command layer (the `--profile` flag is resolved into `App.ProfileName`):
```go
store, _ := config.LoadStore()
name := store.ActiveName(cli.Profile)
cfg := store.Get(name)                       // nil if not configured
apiKey, _, err := config.ResolveAPIKeyFor(name, cfg)
client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
```

## Validation Rules

- **BaseURL** (required): full URL like `https://staging.chatwoot.com`
- **AccountID** (required): numeric account ID from Chatwoot

## File Permissions

Config directory is created with `0700`; the config file is written atomically
(temp file + rename) with `0600`. API keys are never written to YAML.
