# internal/config - Configuration Management

YAML-based configuration persistence for non-secret account settings. Configuration is stored in `~/.chatwoot/config.yaml` and auto-loaded on startup. API keys are resolved from `CHATWOOT_API_KEY` first, then the OS keyring.

## Files

### config.go
Configuration struct and file I/O. Provides:
- `Config` struct with BaseURL and AccountID
- `Load()` — read from `~/.chatwoot/config.yaml`, create if missing
- `Save()` — write non-secret YAML
- Validation: ensures BaseURL and AccountID are set before API calls
- Error handling: distinguishes between missing file and parse errors

### credentials.go
Credential resolution and OS keyring storage. Provides:
- `ResolveAPIKey()` — `CHATWOOT_API_KEY` first, then OS keyring
- `SaveAPIKey()` — write validated login token to keyring
- `DeleteAPIKey()` — remove saved keyring token on logout

## Config Schema

```yaml
base_url: https://staging.chatwoot.com
account_id: 47
```

## Usage

In `main.go`:
```go
cfg, err := config.Load()
if err != nil {
    // Handle missing/invalid config
}

apiKey, _, err := config.ResolveAPIKey(cfg)
if err != nil {
    // Handle missing credentials
}

client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
```

## Validation Rules

- **BaseURL** (required): full URL like `https://staging.chatwoot.com`
- **AccountID** (required): numeric account ID from Chatwoot

## File Permissions

Config directory is created with `0700`; config file is created with `0600`. API keys are not written to YAML.

## TODO

- Implement config migration for schema changes
- Add profile support (multiple saved credentials)
