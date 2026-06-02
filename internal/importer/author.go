package importer

import "strings"

// AuthorResolver maps a source author id to a Chatwoot user id by matching
// emails, falling back to the token owner when no match exists. It tracks how
// many authors were matched vs fell back, for the import summary.
type AuthorResolver struct {
	sourceByID map[string]Author
	cwByEmail  map[string]int
	fallbackID int

	Matched  int
	Fallback int
}

// NewAuthorResolver builds a resolver. cwByEmail keys are lowercased
// defensively so matching is case-insensitive.
func NewAuthorResolver(sourceByID map[string]Author, cwByEmail map[string]int, fallbackID int) *AuthorResolver {
	normalized := make(map[string]int, len(cwByEmail))
	for email, id := range cwByEmail {
		normalized[strings.ToLower(strings.TrimSpace(email))] = id
	}
	return &AuthorResolver{
		sourceByID: sourceByID,
		cwByEmail:  normalized,
		fallbackID: fallbackID,
	}
}

// Resolve returns the Chatwoot user id for a source author, falling back to the
// token owner on any miss (unknown author, no email, or email not an agent).
func (r *AuthorResolver) Resolve(sourceAuthorID string) int {
	if a, ok := r.sourceByID[sourceAuthorID]; ok && a.Email != "" {
		if id, ok := r.cwByEmail[strings.ToLower(strings.TrimSpace(a.Email))]; ok && id != 0 {
			r.Matched++
			return id
		}
	}
	r.Fallback++
	return r.fallbackID
}
