package update

import (
	"testing"
	"time"
)

func TestIsFresh(t *testing.T) {
	tests := []struct {
		name  string
		cache *Cache
		want  bool
	}{
		{"nil cache", nil, false},
		{"just-now", &Cache{CheckedAt: time.Now()}, true},
		{"within ttl", &Cache{CheckedAt: time.Now().Add(-CacheTTL + time.Minute)}, true},
		{"past ttl", &Cache{CheckedAt: time.Now().Add(-CacheTTL - time.Minute)}, false},
		{"future-dated (skew)", &Cache{CheckedAt: time.Now().Add(time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFresh(tt.cache); got != tt.want {
				t.Fatalf("IsFresh = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOutdated(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"", "v1.0.0", false},
		{"dev", "v1.0.0", false},
		{"v1.0.0", "", false},
		{"v1.0.0", "v1.0.0", false},
		{"1.0.0", "v1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v2.0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			if got := IsOutdated(tt.current, tt.latest); got != tt.want {
				t.Fatalf("IsOutdated(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCacheRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got, err := LoadCache(); err != nil || got != nil {
		t.Fatalf("LoadCache() before save = (%v, %v), want (nil, nil)", got, err)
	}

	want := &Cache{LatestVersion: "v1.2.3", CheckedAt: time.Now().UTC().Truncate(time.Second)}
	if err := SaveCache(want); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	got, err := LoadCache()
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if got == nil || got.LatestVersion != want.LatestVersion {
		t.Fatalf("LoadCache() = %#v, want LatestVersion=%q", got, want.LatestVersion)
	}
	if !got.CheckedAt.Equal(want.CheckedAt) {
		t.Fatalf("CheckedAt = %v, want %v", got.CheckedAt, want.CheckedAt)
	}
}

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"dev", "dev"},
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := DisplayVersion(tt.in); got != tt.want {
				t.Fatalf("DisplayVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
