package update

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestFormatNotice(t *testing.T) {
	plain := FormatNotice("v1.0.0", "v1.1.0", false)
	if !strings.Contains(plain, "v1.0.0 → v1.1.0") {
		t.Fatalf("plain notice missing version pair: %q", plain)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain notice should not contain ANSI escapes: %q", plain)
	}

	dim := FormatNotice("v1.0.0", "v1.1.0", true)
	if !strings.Contains(dim, "\x1b[2m") || !strings.Contains(dim, "\x1b[0m") {
		t.Fatalf("dim notice missing ANSI dim/reset escapes: %q", dim)
	}
	if !strings.HasSuffix(dim, "\x1b[0m\n") {
		t.Fatalf("dim notice should end with reset before final newline: %q", dim)
	}
}

func TestFetchLatestFrom(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
				t.Errorf("Accept header = %q, want application/vnd.github+json", got)
			}
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
		}))
		defer srv.Close()

		got, err := fetchLatestFrom(srv.URL)
		if err != nil {
			t.Fatalf("fetchLatestFrom() error = %v", err)
		}
		if got != "v9.9.9" {
			t.Fatalf("fetchLatestFrom() = %q, want v9.9.9", got)
		}
	})

	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "rate limited", http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := fetchLatestFrom(srv.URL)
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("fetchLatestFrom() error = %v, want one containing 403", err)
		}
	})

	t.Run("missing tag_name", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		_, err := fetchLatestFrom(srv.URL)
		if err == nil || !strings.Contains(err.Error(), "tag_name") {
			t.Fatalf("fetchLatestFrom() error = %v, want one mentioning tag_name", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		if _, err := fetchLatestFrom(srv.URL); err == nil {
			t.Fatal("fetchLatestFrom() error = nil, want decode error")
		}
	})

	t.Run("bad url", func(t *testing.T) {
		if _, err := fetchLatestFrom("://not a url"); err == nil {
			t.Fatal("fetchLatestFrom() error = nil for invalid URL")
		}
	})
}

func TestStartRefresh(t *testing.T) {
	t.Run("fresh cache short-circuits", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if err := SaveCache(&Cache{LatestVersion: "v1.0.0", CheckedAt: time.Now()}); err != nil {
			t.Fatalf("SaveCache() error = %v", err)
		}

		var calls int32
		fetch := func() (string, error) {
			atomic.AddInt32(&calls, 1)
			return "v9.0.0", nil
		}
		wait := startRefresh(time.Second, fetch)
		wait()

		if got := atomic.LoadInt32(&calls); got != 0 {
			t.Fatalf("fetch was called %d times, want 0", got)
		}

		cache, _ := LoadCache()
		if cache == nil || cache.LatestVersion != "v1.0.0" {
			t.Fatalf("cache changed unexpectedly: %#v", cache)
		}
	})

	t.Run("stale cache triggers fetch and updates cache", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stale := time.Now().Add(-CacheTTL - time.Hour)
		if err := SaveCache(&Cache{LatestVersion: "v1.0.0", CheckedAt: stale}); err != nil {
			t.Fatalf("SaveCache() error = %v", err)
		}

		fetch := func() (string, error) { return "v9.0.0", nil }
		wait := startRefresh(time.Second, fetch)
		wait()

		cache, err := LoadCache()
		if err != nil {
			t.Fatalf("LoadCache() error = %v", err)
		}
		if cache == nil || cache.LatestVersion != "v9.0.0" {
			t.Fatalf("cache = %#v, want LatestVersion=v9.0.0", cache)
		}
		if !cache.CheckedAt.After(stale) {
			t.Fatalf("CheckedAt not advanced: %v <= %v", cache.CheckedAt, stale)
		}
	})

	t.Run("missing cache triggers fetch", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		fetch := func() (string, error) { return "v2.3.4", nil }
		wait := startRefresh(time.Second, fetch)
		wait()

		cache, _ := LoadCache()
		if cache == nil || cache.LatestVersion != "v2.3.4" {
			t.Fatalf("cache = %#v, want LatestVersion=v2.3.4", cache)
		}
	})

	t.Run("fetch error leaves cache untouched", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		stale := time.Now().Add(-CacheTTL - time.Hour)
		if err := SaveCache(&Cache{LatestVersion: "v1.0.0", CheckedAt: stale}); err != nil {
			t.Fatalf("SaveCache() error = %v", err)
		}

		fetch := func() (string, error) { return "", errors.New("offline") }
		wait := startRefresh(time.Second, fetch)
		wait()

		cache, _ := LoadCache()
		if cache == nil {
			t.Fatal("cache deleted after fetch error")
		}
		if cache.LatestVersion != "v1.0.0" {
			t.Fatalf("LatestVersion = %q, want v1.0.0", cache.LatestVersion)
		}
		// CheckedAt may differ by a sub-nanosecond due to JSON round-trip,
		// but must not have advanced into freshness territory.
		if IsFresh(cache) {
			t.Fatalf("cache became fresh after fetch error: CheckedAt=%v", cache.CheckedAt)
		}
	})

	t.Run("empty fetch result leaves cache untouched", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		fetch := func() (string, error) { return "", nil }
		wait := startRefresh(time.Second, fetch)
		wait()

		cache, _ := LoadCache()
		if cache != nil {
			t.Fatalf("cache written for empty fetch result: %#v", cache)
		}
	})

	t.Run("wait honors timeout when fetch is slow", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		release := make(chan struct{})
		fetch := func() (string, error) {
			<-release
			return "v9.0.0", nil
		}
		wait := startRefresh(20*time.Millisecond, fetch)

		start := time.Now()
		wait()
		elapsed := time.Since(start)
		close(release) // let the goroutine exit so it doesn't leak

		if elapsed > 200*time.Millisecond {
			t.Fatalf("wait took %v, expected ~20ms", elapsed)
		}

		// Cache should NOT yet be populated — the fetch was still blocked.
		cache, _ := LoadCache()
		if cache != nil {
			t.Fatalf("cache populated before fetch returned: %#v", cache)
		}
	})
}

func TestStartRefreshNoNetwork(t *testing.T) {
	// Cover the StartRefresh wrapper without making a real GitHub call:
	// a fresh cache short-circuits before the injected fetcher runs.
	t.Setenv("HOME", t.TempDir())
	if err := SaveCache(&Cache{LatestVersion: "v1.0.0", CheckedAt: time.Now()}); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}
	wait := StartRefresh(10 * time.Millisecond)
	wait() // must be a no-op; nothing to assert beyond "doesn't hang or panic"
}

func TestSaveCacheFailsWhenHomeIsNotADir(t *testing.T) {
	// HOME pointing at a regular file means MkdirAll inside ~/.chatwoot
	// can't succeed regardless of permissions — covers SaveCache's
	// MkdirAll error branch portably (works even when tests run as root).
	tmp := t.TempDir()
	homeFile := filepath.Join(tmp, "home")
	if err := os.WriteFile(homeFile, []byte("not a dir"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", homeFile)

	if err := SaveCache(&Cache{LatestVersion: "v1", CheckedAt: time.Now()}); err == nil {
		t.Fatal("SaveCache() error = nil, want failure when HOME is a file")
	}
}

func TestLoadCacheFailsWhenCachePathIsADir(t *testing.T) {
	// A directory at the cache file path causes ReadFile to return a
	// non-IsNotExist error — covers LoadCache's "read failed" branch.
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".chatwoot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version-cache.json"), 0700); err != nil {
		t.Fatalf("MkdirAll cache-as-dir error = %v", err)
	}

	if _, err := LoadCache(); err == nil {
		t.Fatal("LoadCache() error = nil, want failure when cache path is a directory")
	}
}

func TestLoadCacheMalformedJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".chatwoot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, "version-cache.json")
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadCache(); err == nil {
		t.Fatal("LoadCache() error = nil for malformed JSON")
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
