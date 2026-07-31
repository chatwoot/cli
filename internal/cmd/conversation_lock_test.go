package cmd

import (
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/lock"
)

// Mutating conversation commands must fail fast when another process holds the
// per-conversation lock. The App is empty on purpose: the commands must bail
// out before touching the API client.
func TestConvMutationsFailWhenConversationLocked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)        // unix
	t.Setenv("USERPROFILE", home) // windows

	const convID = 555
	lk, err := lock.AcquireConversation(convID)
	if err != nil {
		t.Fatalf("AcquireConversation: %v", err)
	}
	defer lk.Release()

	app := &App{}
	cases := []struct {
		name string
		run  func() error
	}{
		{"reply", func() error { return (&ConvReplyCmd{ID: convID, Text: "hi"}).Run(app) }},
		{"resolve", func() error { return (&ConvResolveCmd{ID: convID}).Run(app) }},
		{"assign", func() error { return (&ConvAssignCmd{ID: convID, Team: 1}).Run(app) }},
		{"snooze", func() error { return (&ConvSnoozeCmd{ID: convID}).Run(app) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatal("want error while conversation is locked, got nil")
			}
			if !strings.Contains(err.Error(), "already running on this conversation") {
				t.Errorf("error = %q, want lock-held message", err)
			}
		})
	}
}
