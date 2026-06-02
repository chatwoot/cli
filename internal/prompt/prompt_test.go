package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func newTest(input string) (*TermPrompter, *bytes.Buffer) {
	var out bytes.Buffer
	return NewTermPrompter(strings.NewReader(input), &out), &out
}

func TestSelectOne(t *testing.T) {
	p, _ := newTest("2\n")
	idx, err := p.SelectOne("Pick one", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("SelectOne: %v", err)
	}
	if idx != 1 {
		t.Fatalf("idx = %d, want 1", idx)
	}
}

func TestSelectOneRepromptsThenSucceeds(t *testing.T) {
	p, out := newTest("9\nx\n1\n")
	idx, err := p.SelectOne("Pick one", []string{"a", "b"})
	if err != nil {
		t.Fatalf("SelectOne: %v", err)
	}
	if idx != 0 {
		t.Fatalf("idx = %d, want 0", idx)
	}
	if !strings.Contains(out.String(), "between 1 and 2") {
		t.Errorf("expected reprompt message, got: %q", out.String())
	}
}

func TestSelectOneEOFReturnsError(t *testing.T) {
	p, _ := newTest("")
	if _, err := p.SelectOne("Pick", []string{"a"}); err == nil {
		t.Fatal("expected error on EOF")
	}
}

func TestSelectManyCommaList(t *testing.T) {
	p, _ := newTest("1,3\n")
	idxs, err := p.SelectMany("Pick many", []string{"a", "b", "c"}, true)
	if err != nil {
		t.Fatalf("SelectMany: %v", err)
	}
	if len(idxs) != 2 || idxs[0] != 0 || idxs[1] != 2 {
		t.Fatalf("idxs = %v, want [0 2]", idxs)
	}
}

func TestSelectManyAll(t *testing.T) {
	p, _ := newTest("all\n")
	idxs, err := p.SelectMany("Pick many", []string{"a", "b", "c"}, true)
	if err != nil {
		t.Fatalf("SelectMany: %v", err)
	}
	if len(idxs) != 3 {
		t.Fatalf("idxs = %v, want all three", idxs)
	}
}

func TestSelectManyDedupsAndReprompts(t *testing.T) {
	p, _ := newTest("5\n2,2,1\n")
	idxs, err := p.SelectMany("Pick", []string{"a", "b", "c"}, false)
	if err != nil {
		t.Fatalf("SelectMany: %v", err)
	}
	if len(idxs) != 2 || idxs[0] != 1 || idxs[1] != 0 {
		t.Fatalf("idxs = %v, want [1 0] (deduped, ordered)", idxs)
	}
}

func TestInputDefaultOnEmpty(t *testing.T) {
	p, _ := newTest("\n")
	got, err := p.Input("Slug", "acme-support")
	if err != nil {
		t.Fatalf("Input: %v", err)
	}
	if got != "acme-support" {
		t.Fatalf("got %q, want default", got)
	}
}

func TestInputOverride(t *testing.T) {
	p, _ := newTest("custom-slug\n")
	got, err := p.Input("Slug", "acme-support")
	if err != nil {
		t.Fatalf("Input: %v", err)
	}
	if got != "custom-slug" {
		t.Fatalf("got %q, want custom-slug", got)
	}
}

func TestConfirm(t *testing.T) {
	cases := map[string]bool{
		"y\n":    true,
		"yes\n":  true,
		"Y\n":    true,
		"\n":     false,
		"n\n":    false,
		"nope\n": false,
	}
	for in, want := range cases {
		p, _ := newTest(in)
		got, err := p.Confirm("Proceed?")
		if err != nil {
			t.Fatalf("Confirm(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("Confirm(%q) = %v, want %v", in, got, want)
		}
	}
}
