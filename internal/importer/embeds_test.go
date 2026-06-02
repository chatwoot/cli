package importer

import "testing"

func TestEmbedRegistryCanonicalizes(t *testing.T) {
	reg := DefaultEmbedRegistry()
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.youtube.com/embed/abc123", "https://www.youtube.com/watch?v=abc123"},
		{"https://www.youtube-nocookie.com/embed/abc123", "https://www.youtube.com/watch?v=abc123"},
		{"https://youtu.be/abc123", "https://www.youtube.com/watch?v=abc123"},
		{"https://www.youtube.com/watch?v=abc123", "https://www.youtube.com/watch?v=abc123"},
		{"https://www.loom.com/embed/deadbeef", "https://www.loom.com/share/deadbeef"},
		{"https://player.vimeo.com/video/123456", "https://vimeo.com/123456"},
		{"https://fast.wistia.net/embed/iframe/xyz", "https://fast.wistia.com/medias/xyz"},
		{"https://app.arcade.software/embed/flow1", "https://app.arcade.software/share/flow1"},
		{"https://www.guidejar.com/embed/guide1", "https://www.guidejar.com/guides/guide1"},
		{"https://codepen.io/jdoe/embed/AbCdEf", "https://codepen.io/jdoe/pen/AbCdEf"},
		{"https://iframe.mediadelivery.net/embed/42/vid7", "https://iframe.mediadelivery.net/play/42/vid7"},
		{"https://gist.github.com/jdoe/abc123def456", "https://gist.github.com/jdoe/abc123def456"},
		{"https://downloads.intercomcdn.com/clip.mp4", "https://downloads.intercomcdn.com/clip.mp4"},
	}
	for _, tc := range cases {
		got, ok := reg.Match(tc.in)
		if !ok {
			t.Errorf("Match(%q) not matched", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("Match(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEmbedRegistryRejectsUnknown(t *testing.T) {
	reg := DefaultEmbedRegistry()
	if got, ok := reg.Match("https://example.com/page"); ok {
		t.Errorf("unexpected match: %q", got)
	}
}
