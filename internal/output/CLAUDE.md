# internal/output - Output Formatting

Multi-format output printer for CLI commands. Supports text (tabwriter), JSON, CSV, and quiet modes.

## Files

### printer.go
Main Printer struct and formatting logic. Provides:
- `Printer` struct with format preference (text/json/csv/quiet)
- `Print()` — generic output that respects format flag
- `PrintTable()` — render structs as tabwriter tables
- `PrintJSON()` — pretty-print as JSON
- `PrintCSV()` — comma-separated values (all fields)
- `Quiet()` — suppress output (used in some commands)

## Format Modes

**Text (default):**
- Uses Go's `tabwriter` for aligned columns
- Struct tags `table:"header"` define column names
- Suitable for human reading in terminal

**JSON:**
- Pretty-printed with indentation
- Useful for piping to other tools
- Set via `--format json`

**CSV:**
- All fields exported as comma-separated values
- Includes header row
- Suitable for import to spreadsheets
- Set via `--format csv`

**Quiet:**
- Suppresses output entirely
- Useful for scripts checking exit codes
- Set via `--quiet` flag

## Usage in Commands

```go
func (c *MyCommand) Run(app *App) error {
    data, _ := app.Client.SomeService().List()
    return app.Printer.Print(data)
}
```

The Printer automatically selects format based on initialization.

## Table Struct Tags

Define output columns with struct tags:
```go
type Item struct {
    ID   int    `table:"ID"`
    Name string `table:"Name"`
    Status string `table:"Status"`
}
```

## TODO

- Add table header customization per command
- Implement YAML output format
- Add column selection (--columns id,name)
- Implement filtering for table output
