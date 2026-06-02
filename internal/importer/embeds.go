package importer

import "regexp"

// EmbedRegistry maps a provider embed/iframe URL back to the canonical "bare"
// URL form that Chatwoot's markdown embed registry (config/markdown_embeds.yml)
// recognizes and expands into an iframe at render time. Intercom stores rich
// media as <iframe src="...embed..."> tags; Chatwoot instead wants the bare
// share/watch URL on its own line, so we invert the embed URL here.
type EmbedRegistry struct {
	matchers []embedMatcher
}

type embedMatcher struct {
	name  string
	re    *regexp.Regexp
	build func(groups []string) string
}

// Match returns the canonical bare URL for url if it matches a known provider.
func (r EmbedRegistry) Match(url string) (string, bool) {
	for _, m := range r.matchers {
		if g := m.re.FindStringSubmatch(url); g != nil {
			return m.build(g), true
		}
	}
	return "", false
}

// DefaultEmbedRegistry returns the provider matchers in priority order. The
// canonical output forms are aligned with the regexes in Chatwoot's
// config/markdown_embeds.yml (mp4 is last as its pattern is the broadest).
func DefaultEmbedRegistry() EmbedRegistry {
	return EmbedRegistry{matchers: []embedMatcher{
		{
			name: "youtube",
			re:   regexp.MustCompile(`(?i)(?:youtube(?:-nocookie)?\.com/(?:embed/|watch\?v=)|youtu\.be/)([A-Za-z0-9_\-]+)`),
			build: func(g []string) string {
				return "https://www.youtube.com/watch?v=" + g[1]
			},
		},
		{
			name:  "loom",
			re:    regexp.MustCompile(`(?i)loom\.com/(?:embed|share)/([A-Za-z0-9]+)`),
			build: func(g []string) string { return "https://www.loom.com/share/" + g[1] },
		},
		{
			name:  "vimeo",
			re:    regexp.MustCompile(`(?i)(?:player\.)?vimeo\.com/(?:video/)?(\d+)`),
			build: func(g []string) string { return "https://vimeo.com/" + g[1] },
		},
		{
			name: "wistia",
			re:   regexp.MustCompile(`(?i)([A-Za-z0-9\-]+)\.wistia\.(?:com|net)/(?:medias/|embed/(?:iframe/|medias/))([A-Za-z0-9]+)`),
			build: func(g []string) string {
				return "https://" + g[1] + ".wistia.com/medias/" + g[2]
			},
		},
		{
			name:  "arcade",
			re:    regexp.MustCompile(`(?i)app\.arcade\.software/(?:share|embed)/([A-Za-z0-9\-]+)`),
			build: func(g []string) string { return "https://app.arcade.software/share/" + g[1] },
		},
		{
			name:  "guidejar",
			re:    regexp.MustCompile(`(?i)guidejar\.com/(?:embed|guides)/([A-Za-z0-9\-]+)`),
			build: func(g []string) string { return "https://www.guidejar.com/guides/" + g[1] },
		},
		{
			name: "codepen",
			re:   regexp.MustCompile(`(?i)codepen\.io/([^/]+)/(?:embed|pen)/([A-Za-z0-9\-]+)`),
			build: func(g []string) string {
				return "https://codepen.io/" + g[1] + "/pen/" + g[2]
			},
		},
		{
			name: "bunny",
			re:   regexp.MustCompile(`(?i)(?:iframe|player)\.mediadelivery\.net/(?:play|embed)/(\d+)/([A-Za-z0-9\-]+)`),
			build: func(g []string) string {
				return "https://iframe.mediadelivery.net/play/" + g[1] + "/" + g[2]
			},
		},
		{
			name: "github_gist",
			re:   regexp.MustCompile(`(?i)gist\.github\.com/([^/]+)/([a-f0-9]+)`),
			build: func(g []string) string {
				return "https://gist.github.com/" + g[1] + "/" + g[2]
			},
		},
		{
			name:  "mp4",
			re:    regexp.MustCompile(`(?i)(https?://[^\s"'<>]+\.mp4)`),
			build: func(g []string) string { return g[1] },
		},
	}}
}
