package importer

import "testing"

func TestAuthorResolverMatchesByEmailCaseInsensitive(t *testing.T) {
	source := map[string]Author{
		"i1": {ID: "i1", Email: "Ada@Acme.com", Name: "Ada"},
	}
	cw := map[string]int{"ada@acme.com": 42}
	r := NewAuthorResolver(source, cw, 1)

	if got := r.Resolve("i1"); got != 42 {
		t.Errorf("Resolve = %d, want 42", got)
	}
	if r.Matched != 1 || r.Fallback != 0 {
		t.Errorf("matched=%d fallback=%d, want 1/0", r.Matched, r.Fallback)
	}
}

func TestAuthorResolverFallsBack(t *testing.T) {
	source := map[string]Author{
		"i1": {ID: "i1", Email: "nobody@elsewhere.com"},
		"i2": {ID: "i2", Email: ""}, // no email
	}
	cw := map[string]int{"ada@acme.com": 42}
	r := NewAuthorResolver(source, cw, 7)

	if got := r.Resolve("i1"); got != 7 { // email not an agent
		t.Errorf("Resolve(i1) = %d, want fallback 7", got)
	}
	if got := r.Resolve("i2"); got != 7 { // no email
		t.Errorf("Resolve(i2) = %d, want fallback 7", got)
	}
	if got := r.Resolve("unknown"); got != 7 { // unknown author id
		t.Errorf("Resolve(unknown) = %d, want fallback 7", got)
	}
	if r.Fallback != 3 || r.Matched != 0 {
		t.Errorf("matched=%d fallback=%d, want 0/3", r.Matched, r.Fallback)
	}
}
