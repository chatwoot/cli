// Package update centralizes the "what is the latest released version?"
// logic and the on-disk cache that backs the per-invocation outdated
// check.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chatwoot/cli/internal/config"
)

// ANSI escape sequences used to render the outdated-version notice as
// muted (dim) text. The reset is applied at the end of every line so a
// truncated print (e.g. broken pipe) can't leak dim styling into the
// user's prompt.
const (
	ansiDim   = "\x1b[2m"
	ansiReset = "\x1b[0m"
)

// LatestReleaseURL is the GitHub API endpoint for the most recent release.
const LatestReleaseURL = "https://api.github.com/repos/chatwoot/cli/releases/latest"

// CacheTTL is how long a cached "latest version" answer is trusted before
// a fresh fetch is attempted.
const CacheTTL = 24 * time.Hour

// fetchTimeout caps a single GitHub API call. The background refresh wait
// in main is shorter so we never block the CLI exit on a slow network.
const fetchTimeout = 5 * time.Second

// Cache is the on-disk record of the last successful GitHub lookup.
type Cache struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

func cachePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "version-cache.json"), nil
}

// LoadCache reads the cache file. A missing file returns (nil, nil) so
// callers can treat first-run and stale-cache uniformly.
func LoadCache() (*Cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveCache writes the cache atomically-enough for our purposes (a single
// short JSON blob; partial writes are caught by the next Unmarshal).
func SaveCache(c *Cache) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path, err := cachePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// IsFresh reports whether the cache is non-nil and younger than CacheTTL.
// A future-dated CheckedAt (clock skew) is treated as fresh.
func IsFresh(c *Cache) bool {
	if c == nil {
		return false
	}
	return time.Since(c.CheckedAt) < CacheTTL
}

// FetchLatest hits GitHub for the most recent release tag.
func FetchLatest() (string, error) {
	return fetchLatestFrom(LatestReleaseURL)
}

// fetchLatestFrom is the workhorse — split out so tests can point at an
// httptest.Server instead of the real GitHub endpoint.
func fetchLatestFrom(url string) (string, error) {
	client := &http.Client{Timeout: fetchTimeout}
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

// StartRefresh returns a wait function the caller can invoke before exit.
// If the cache is fresh, no fetch is started and wait is a no-op. Otherwise
// a goroutine fetches and updates the cache; wait blocks up to waitFor for
// it to finish so the cache is current for the next invocation.
//
// We deliberately use stale cache data for the *current* run so the user
// never pays the network cost on the hot path.
func StartRefresh(waitFor time.Duration) func() {
	return startRefresh(waitFor, FetchLatest)
}

// startRefresh is the testable core of StartRefresh — fetch is injected
// so tests can avoid the real GitHub round-trip.
func startRefresh(waitFor time.Duration, fetch func() (string, error)) func() {
	cache, _ := LoadCache()
	if IsFresh(cache) {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		latest, err := fetch()
		if err != nil || latest == "" {
			return
		}
		_ = SaveCache(&Cache{LatestVersion: latest, CheckedAt: time.Now()})
	}()
	return func() {
		select {
		case <-done:
		case <-time.After(waitFor):
		}
	}
}

// IsOutdated returns true when the current build is a tagged release and
// differs from the latest tag. Dev builds and empty inputs always report
// "not outdated" so we never nag users running unreleased binaries.
func IsOutdated(current, latest string) bool {
	if current == "" || current == "dev" || latest == "" {
		return false
	}
	return Normalize(current) != Normalize(latest)
}

// Normalize strips a leading "v" and surrounding whitespace so "v1.2.3"
// and "1.2.3" compare equal.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// DisplayVersion renders a version for human output, prepending "v" for
// tagged releases and leaving "dev"/empty as-is.
func DisplayVersion(v string) string {
	if v == "" || v == "dev" {
		return v
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// FormatNotice builds the two-line outdated-version banner. When color is
// true each line is wrapped in ANSI dim + reset so the notice reads as
// secondary, non-urgent information.
func FormatNotice(current, latest string, color bool) string {
	lines := []string{
		fmt.Sprintf("A new version of chatwoot is available: %s → %s",
			DisplayVersion(current), DisplayVersion(latest)),
		"  curl -fsSL https://chwt.app/install-cli | sh",
	}
	var b strings.Builder
	b.WriteByte('\n')
	for _, line := range lines {
		if color {
			b.WriteString(ansiDim)
			b.WriteString(line)
			b.WriteString(ansiReset)
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
