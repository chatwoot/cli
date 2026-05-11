package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/output"
)

// stubFetch swaps fetchLatest for the duration of a test.
func stubFetch(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := fetchLatest
	fetchLatest = fn
	t.Cleanup(func() { fetchLatest = orig })
}

func newTestApp(t *testing.T, version string) (*App, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	printer := output.NewPrinter("text", false, false)
	printer.Writer = &out
	return &App{Printer: printer, Version: version}, &out
}

func TestVersionCmdPrintsVersion(t *testing.T) {
	app, out := newTestApp(t, "v1.2.3")

	if err := (&VersionCmd{}).Run(app); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "v1.2.3" {
		t.Fatalf("output = %q, want v1.2.3", got)
	}
}

func TestVersionCmdPrintsDevWhenVersionEmpty(t *testing.T) {
	app, out := newTestApp(t, "")

	if err := (&VersionCmd{}).Run(app); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "dev" {
		t.Fatalf("output = %q, want dev", got)
	}
}

func TestVersionCmdCheckUpToDate(t *testing.T) {
	stubFetch(t, func() (string, error) { return "v1.2.3", nil })
	app, out := newTestApp(t, "v1.2.3")

	if err := (&VersionCmd{Check: true}).Run(app); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "chatwoot v1.2.3") {
		t.Fatalf("missing current version line: %q", got)
	}
	if !strings.Contains(got, "Latest: v1.2.3") {
		t.Fatalf("missing latest line: %q", got)
	}
	if !strings.Contains(got, "Up to date.") {
		t.Fatalf("missing up-to-date message: %q", got)
	}
	if strings.Contains(got, "Update available") {
		t.Fatalf("up-to-date output mentioned update: %q", got)
	}
}

func TestVersionCmdCheckOutdated(t *testing.T) {
	stubFetch(t, func() (string, error) { return "v2.0.0", nil })
	app, out := newTestApp(t, "v1.2.3")

	if err := (&VersionCmd{Check: true}).Run(app); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Latest: v2.0.0") {
		t.Fatalf("missing latest line: %q", got)
	}
	if !strings.Contains(got, "Update available.") {
		t.Fatalf("missing update-available message: %q", got)
	}
	if !strings.Contains(got, "curl -fsSL https://chwt.app/install-cli | sh") {
		t.Fatalf("missing install hint: %q", got)
	}
}

func TestVersionCmdCheckDevBuild(t *testing.T) {
	stubFetch(t, func() (string, error) { return "v9.9.9", nil })
	app, out := newTestApp(t, "dev")

	if err := (&VersionCmd{Check: true}).Run(app); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Running a dev build.") {
		t.Fatalf("missing dev-build message: %q", got)
	}
	// A dev build should never get nagged to upgrade.
	if strings.Contains(got, "Update available") {
		t.Fatalf("dev build was told to upgrade: %q", got)
	}
}

func TestVersionCmdCheckFetchError(t *testing.T) {
	stubFetch(t, func() (string, error) { return "", errors.New("network down") })
	app, _ := newTestApp(t, "v1.0.0")

	err := (&VersionCmd{Check: true}).Run(app)
	if err == nil {
		t.Fatal("Run() error = nil, want fetch failure")
	}
	if !strings.Contains(err.Error(), "check failed") || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("error = %v, want wrapped fetch failure", err)
	}
}
