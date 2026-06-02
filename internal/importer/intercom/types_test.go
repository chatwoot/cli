package intercom

import (
	"encoding/json"
	"testing"
)

func TestNextCursorObjectForm(t *testing.T) {
	p := &pages{Next: json.RawMessage(`{"page":2,"starting_after":"abc123"}`)}
	if got := p.nextCursor(); got != "abc123" {
		t.Fatalf("nextCursor = %q, want abc123", got)
	}
}

func TestNextCursorURLFormDecodesPercentEncoding(t *testing.T) {
	// A full-URL cursor whose starting_after is percent-encoded (%3D == "=").
	// The old substring extraction returned it still-encoded, which got
	// double-encoded on the next request and skipped pages.
	p := &pages{Next: json.RawMessage(`"https://api.intercom.io/articles?per_page=150&starting_after=WzE3MF0%3D"`)}
	if got := p.nextCursor(); got != "WzE3MF0=" {
		t.Fatalf("nextCursor = %q, want WzE3MF0= (decoded)", got)
	}
}

func TestNextCursorBareQueryForm(t *testing.T) {
	p := &pages{Next: json.RawMessage(`"starting_after=a%2Fb"`)}
	if got := p.nextCursor(); got != "a/b" {
		t.Fatalf("nextCursor = %q, want a/b (decoded)", got)
	}
}

func TestNextCursorNullOrEmpty(t *testing.T) {
	if got := (&pages{Next: json.RawMessage(`null`)}).nextCursor(); got != "" {
		t.Errorf("null next = %q, want empty", got)
	}
	if got := (&pages{}).nextCursor(); got != "" {
		t.Errorf("absent next = %q, want empty", got)
	}
}
