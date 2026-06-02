package intercom

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetRetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errors":[{"code":"rate_limit_exceeded"}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"admin","id":"42","email":"me@acme.com","app":{"id_code":"ws_123"}}`))
	}))
	t.Cleanup(server.Close)

	var slept time.Duration
	c := NewClient(server.URL, "tok",
		WithHTTPClient(server.Client()),
		withSleep(func(d time.Duration) { slept += d }),
	)

	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.App.IDCode != "ws_123" {
		t.Fatalf("id_code = %q, want ws_123", me.App.IDCode)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d, want 2 (one retry)", calls)
	}
	if slept != time.Second {
		t.Errorf("slept = %v, want 1s (honored Retry-After)", slept)
	}
}

func TestGetSetsAuthAndVersionHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		if got := r.Header.Get("Intercom-Version"); got != "2.13" {
			t.Errorf("Intercom-Version = %q, want 2.13", got)
		}
		_, _ = w.Write([]byte(`{"type":"admin","id":"1","app":{"id_code":"w"}}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "tok", WithHTTPClient(server.Client()), WithVersion("2.13"))
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: %v", err)
	}
}

func TestFetchListPaginatesWithStartingAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("starting_after") {
		case "":
			_, _ = fmt.Fprint(w, `{"type":"list","data":[{"id":"1"},{"id":"2"}],
				"pages":{"next":{"page":2,"starting_after":"CURSOR2"},"per_page":2},"total_count":3}`)
		case "CURSOR2":
			_, _ = fmt.Fprint(w, `{"type":"list","data":[{"id":"3"}],
				"pages":{"next":null,"per_page":2},"total_count":3}`)
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "tok", WithHTTPClient(server.Client()))
	got, err := fetchList[helpCenter](context.Background(), c, "/help_center/help_centers")
	if err != nil {
		t.Fatalf("fetchList: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3 (across 2 pages)", len(got))
	}
	if got[2].ID.String() != "3" {
		t.Errorf("last id = %q, want 3", got[2].ID.String())
	}
}

func TestGetReturnsErrorOn4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"code":"unauthorized"}]}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "bad", WithHTTPClient(server.Client()))
	if _, err := c.Me(context.Background()); err == nil {
		t.Fatal("expected error on 401")
	}
}
