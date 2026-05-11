// Package update centralizes the "what is the latest released version?"
// logic so the version command and any future outdated-check share one
// source of truth.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LatestReleaseURL is the GitHub API endpoint for the most recent release.
const LatestReleaseURL = "https://api.github.com/repos/chatwoot/cli/releases/latest"

// fetchTimeout caps a single GitHub API call.
const fetchTimeout = 5 * time.Second

// FetchLatest hits GitHub for the most recent release tag.
func FetchLatest() (string, error) {
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequest(http.MethodGet, LatestReleaseURL, nil)
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
