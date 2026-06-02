package importer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStateKeyIsStableAndCoordinateSensitive(t *testing.T) {
	a := StateKey("intercom", "ws1", "hc1", "acme-support")
	b := StateKey("intercom", "ws1", "hc1", "acme-support")
	if a != b {
		t.Fatalf("key not stable: %q != %q", a, b)
	}
	if a == StateKey("intercom", "ws1", "hc1", "other-portal") {
		t.Fatal("different target slug should produce a different key")
	}
	if len(a) != 16 {
		t.Fatalf("key length = %d, want 16", len(a))
	}
}

func TestLoadStateMissingReturnsNil(t *testing.T) {
	s, err := LoadState(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil state for missing file, got %#v", s)
	}
}

func TestStateSaveLoadRoundTripAndPerms(t *testing.T) {
	dir := t.TempDir()
	path := StatePath(dir, "key123")

	s := NewState("intercom", "ws1", "hc1")
	s.TargetPortal = PortalRef{Slug: "acme-support", ID: 7}
	s.Locales = []string{"en", "fr"}
	s.SetCategory("coll1", "en", ItemRef{ID: 10, Slug: "faq"})
	s.SetArticle("art1", "en", ItemRef{ID: 100, Slug: "hello"})

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("perm = %o, want 600", perm)
		}
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded state")
	}
	if ref, ok := loaded.CategoryRef("coll1", "en"); !ok || ref.ID != 10 {
		t.Errorf("category ref = %#v ok=%v, want id 10", ref, ok)
	}
	if ref, ok := loaded.ArticleRef("art1", "en"); !ok || ref.ID != 100 {
		t.Errorf("article ref = %#v ok=%v, want id 100", ref, ok)
	}
	if loaded.TargetPortal.ID != 7 {
		t.Errorf("portal id = %d, want 7", loaded.TargetPortal.ID)
	}
}

func TestSetCategoryClearsFailure(t *testing.T) {
	s := NewState("intercom", "ws1", "hc1")
	key := categoryKey("coll1", "fr")
	s.RecordFailure(key, "category", "boom")
	if _, ok := s.Failures[key]; !ok {
		t.Fatal("failure not recorded")
	}
	s.SetCategory("coll1", "fr", ItemRef{ID: 5})
	if _, ok := s.Failures[key]; ok {
		t.Fatal("failure should be cleared after success")
	}
}

func TestMissingRefsReturnFalse(t *testing.T) {
	s := NewState("intercom", "ws1", "hc1")
	if _, ok := s.CategoryRef("nope", "en"); ok {
		t.Error("expected no category ref")
	}
	if _, ok := s.ArticleRef("nope", "en"); ok {
		t.Error("expected no article ref")
	}
}
