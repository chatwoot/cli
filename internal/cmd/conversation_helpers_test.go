package cmd

import (
	"testing"
	"time"

	"github.com/chatwoot/cli/internal/sdk"
)

func TestParseExtendedDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"3d", 3 * 24 * time.Hour, true},
		{"1d", 24 * time.Hour, true},
		{"2w", 14 * 24 * time.Hour, true},
		{"24h", 0, false}, // 'h' is not extended; stdlib handles it
		{"d", 0, false},   // missing number
		{"", 0, false},
		{"abc", 0, false},
		{"5x", 0, false}, // unknown unit
	}
	for _, tc := range cases {
		got, ok := parseExtendedDuration(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseExtendedDuration(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseSnoozeUntil(t *testing.T) {
	now := time.Now()

	t.Run("stdlib duration", func(t *testing.T) {
		ts, err := parseSnoozeUntil("2h")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		delta := ts - now.Add(2*time.Hour).Unix()
		if delta < -2 || delta > 2 {
			t.Errorf("2h offset off by %ds", delta)
		}
	})

	t.Run("extended duration days", func(t *testing.T) {
		ts, err := parseSnoozeUntil("3d")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		delta := ts - now.Add(3*24*time.Hour).Unix()
		if delta < -2 || delta > 2 {
			t.Errorf("3d offset off by %ds", delta)
		}
	})

	t.Run("absolute date", func(t *testing.T) {
		ts, err := parseSnoozeUntil("2026-05-10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want, _ := time.Parse("2006-01-02", "2026-05-10")
		if ts != want.Unix() {
			t.Errorf("got %d, want %d", ts, want.Unix())
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := parseSnoozeUntil("not-a-time"); err == nil {
			t.Fatal("expected error for invalid input")
		}
	})
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "ab..."},
	}
	for _, tc := range cases {
		if got := truncate(tc.in, tc.max); got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestMessageTypeName(t *testing.T) {
	cases := map[int]string{
		0:  "incoming",
		1:  "outgoing",
		2:  "activity",
		99: "99",
	}
	for in, want := range cases {
		if got := messageTypeName(in); got != want {
			t.Errorf("messageTypeName(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	if got := formatTimestamp(0); got != "" {
		t.Errorf("formatTimestamp(0) = %q, want empty", got)
	}
	ts, _ := time.Parse(time.RFC3339, "2026-05-10T12:34:00Z")
	got := formatTimestamp(ts.Unix())
	if got == "" {
		t.Error("non-zero timestamp should format non-empty")
	}
}

func TestSenderName(t *testing.T) {
	if got := senderName(sdk.Conversation{}); got != "" {
		t.Errorf("nil sender = %q, want empty", got)
	}
	conv := sdk.Conversation{Meta: sdk.ConversationMeta{Sender: &sdk.Contact{Name: "Ada"}}}
	if got := senderName(conv); got != "Ada" {
		t.Errorf("got %q, want Ada", got)
	}
}

func TestAssigneeName(t *testing.T) {
	if got := assigneeName(sdk.Conversation{}); got != "" {
		t.Errorf("nil assignee = %q, want empty", got)
	}
	conv := sdk.Conversation{Meta: sdk.ConversationMeta{Assignee: &sdk.Agent{Name: "Grace"}}}
	if got := assigneeName(conv); got != "Grace" {
		t.Errorf("got %q, want Grace", got)
	}
}

func TestLabelsString(t *testing.T) {
	if got := labelsString(nil); got != "" {
		t.Errorf("nil labels = %q, want empty", got)
	}
	if got := labelsString([]string{"a", "b"}); got != "a, b" {
		t.Errorf("got %q, want %q", got, "a, b")
	}
}
