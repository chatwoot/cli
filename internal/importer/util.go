package importer

import (
	"regexp"
	"strings"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a name into a URL-safe slug: lowercase, non-alphanumeric
// runs collapsed to single hyphens, trimmed. Returns "category" for input that
// reduces to empty so the (required) category slug is never blank.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "category"
	}
	return s
}

// unionLocales returns the de-duplicated union of locale lists, preserving the
// order of first appearance.
func unionLocales(lists ...[]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, list := range lists {
		for _, l := range list {
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}
