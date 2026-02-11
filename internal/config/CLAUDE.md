# internal/config - Configuration Management

YAML-based configuration persistence for API credentials and account settings. Configuration is stored in `~/.chatwoot/config.yaml` and auto-loaded on startup.

## Files

### config.go
Configuration struct and file I/O. Provides:
- `Config` struct with BaseURL, Token, AccountID
- `Load()` — read from `~/.chatwoot/config.yaml`, create if missing
- `Save()` — write to YAML with atomic file operations
- Validation: ensures Token and BaseURL are set before API calls
- Error handling: distinguishes between missing file and parse errors

## Config Schema

```yaml
base_url: https://staging.chatwoot.com
token: cwt_abcd1234...
account_id: 47
```

## Usage

In `main.go`:
```go
cfg, err := config.Load()
if err != nil {
    // Handle missing/invalid config for CLI/TUI
}

client := sdk.NewClient(cfg.BaseURL, cfg.Token)
```

In TUI:
```go
tui.Run(client, cfg.AccountID, version)
```

## Validation Rules

- **Token** (required): starts with `cwt_`, stored as-is
- **BaseURL** (required): full URL like `https://staging.chatwoot.com`
- **AccountID** (optional for CLI, required for TUI): numeric account ID from Chatwoot

## File Permissions

Config file created with `0600` (read/write by owner only) to protect sensitive token.

## TODO

- Add config encryption for stored token
- Support environment variable override (CWT_TOKEN, CWT_BASE_URL)
- Implement config migration for schema changes
- Add profile support (multiple saved credentials)
