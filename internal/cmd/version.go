package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/chatwoot/cli/releases/latest"

// VersionCmd is `chatwoot version` — prints the CLI version. Equivalent to
// the `--version` flag, exposed as a subcommand for discoverability.
type VersionCmd struct {
	Check bool `help:"Check GitHub for a newer release."`
}

func (c *VersionCmd) Run(app *App) error {
	v := app.Version
	if v == "" {
		v = "dev"
	}
	if !c.Check {
		_, err := fmt.Fprintln(app.Printer.Writer, v)
		return err
	}

	latest, err := fetchLatestRelease(latestReleaseURL)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "chatwoot %s\n", displayVersion(v))
	fmt.Fprintf(&b, "Latest: %s\n\n", displayVersion(latest))
	switch {
	case v == "dev":
		b.WriteString("Running a dev build.\n")
	case normalizeVersion(v) == normalizeVersion(latest):
		b.WriteString("Up to date.\n")
	default:
		b.WriteString("Update available.\n")
		b.WriteString("  curl -fsSL https://chwt.app/install-cli | sh\n")
	}
	_, err = fmt.Fprint(app.Printer.Writer, b.String())
	return err
}

func displayVersion(v string) string {
	if v == "dev" || v == "" {
		return v
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func fetchLatestRelease(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.TagName == "" {
		return "", fmt.Errorf("no tag_name in response")
	}
	return body.TagName, nil
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}
