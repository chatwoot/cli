package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stateVersion is the on-disk schema version.
const stateVersion = 1

// ItemRef is a created Chatwoot record (category or article).
type ItemRef struct {
	ID   int    `json:"id"`
	Slug string `json:"slug,omitempty"`
}

// FailureRec records a per-item failure for reporting and retry.
type FailureRec struct {
	Kind  string `json:"kind"`
	Error string `json:"error"`
	At    string `json:"at"`
}

// PortalRef is the resolved target portal.
type PortalRef struct {
	Slug string `json:"slug"`
	ID   int    `json:"id"`
}

// State is the resumable import state persisted under ~/.chatwoot/imports/.
// Categories and Articles map a source id -> locale -> created Chatwoot record.
type State struct {
	Version      int                           `json:"version"`
	Provider     string                        `json:"provider"`
	WorkspaceID  string                        `json:"workspace_id"`
	SourceHCID   string                        `json:"source_help_center_id"`
	TargetPortal PortalRef                     `json:"target_portal"`
	Locales      []string                      `json:"locales"`
	Categories   map[string]map[string]ItemRef `json:"categories"`
	Articles     map[string]map[string]ItemRef `json:"articles"`
	Failures     map[string]FailureRec         `json:"failures"`
	UpdatedAt    string                        `json:"updated_at"`
}

// NewState returns an initialized, empty state.
func NewState(provider, workspaceID, sourceHCID string) *State {
	return &State{
		Version:     stateVersion,
		Provider:    provider,
		WorkspaceID: workspaceID,
		SourceHCID:  sourceHCID,
		Categories:  map[string]map[string]ItemRef{},
		Articles:    map[string]map[string]ItemRef{},
		Failures:    map[string]FailureRec{},
	}
}

// StateKey derives a stable file key from the import coordinates. Target slug
// is known before any write (portal slugs are used verbatim by Chatwoot).
func StateKey(provider, workspaceID, sourceHCID, targetSlug string) string {
	sum := sha256.Sum256([]byte(provider + ":" + workspaceID + ":" + sourceHCID + ":" + targetSlug))
	return hex.EncodeToString(sum[:])[:16]
}

// StatePath returns the on-disk path for a state key within dir.
func StatePath(dir, key string) string {
	return filepath.Join(dir, key+".json")
}

// LoadState reads state from path, returning nil (not an error) when the file
// does not exist.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read import state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse import state: %w", err)
	}
	s.ensureMaps()
	return &s, nil
}

func (s *State) ensureMaps() {
	if s.Categories == nil {
		s.Categories = map[string]map[string]ItemRef{}
	}
	if s.Articles == nil {
		s.Articles = map[string]map[string]ItemRef{}
	}
	if s.Failures == nil {
		s.Failures = map[string]FailureRec{}
	}
}

// Save writes state atomically (tmp + rename) with 0600 perms, creating the
// parent directory (0700) if needed.
func (s *State) Save(path string) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create import state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write import state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to commit import state: %w", err)
	}
	return nil
}

// CategoryRef returns the recorded category for a collection+locale, if any.
func (s *State) CategoryRef(collectionID, locale string) (ItemRef, bool) {
	byLocale, ok := s.Categories[collectionID]
	if !ok {
		return ItemRef{}, false
	}
	ref, ok := byLocale[locale]
	return ref, ok
}

// SetCategory records a created category and clears any prior failure for it.
func (s *State) SetCategory(collectionID, locale string, ref ItemRef) {
	if s.Categories[collectionID] == nil {
		s.Categories[collectionID] = map[string]ItemRef{}
	}
	s.Categories[collectionID][locale] = ref
	delete(s.Failures, categoryKey(collectionID, locale))
}

// ArticleRef returns the recorded article for an article+locale, if any.
func (s *State) ArticleRef(articleID, locale string) (ItemRef, bool) {
	byLocale, ok := s.Articles[articleID]
	if !ok {
		return ItemRef{}, false
	}
	ref, ok := byLocale[locale]
	return ref, ok
}

// SetArticle records a created article and clears any prior failure for it.
func (s *State) SetArticle(articleID, locale string, ref ItemRef) {
	if s.Articles[articleID] == nil {
		s.Articles[articleID] = map[string]ItemRef{}
	}
	s.Articles[articleID][locale] = ref
	delete(s.Failures, articleKey(articleID, locale))
}

// RecordFailure stores a per-item failure keyed by its composite id.
func (s *State) RecordFailure(key, kind, errMsg string) {
	s.Failures[key] = FailureRec{Kind: kind, Error: errMsg, At: time.Now().UTC().Format(time.RFC3339)}
}

func categoryKey(collectionID, locale string) string {
	return "category:" + collectionID + ":" + locale
}
func articleKey(articleID, locale string) string { return "article:" + articleID + ":" + locale }
