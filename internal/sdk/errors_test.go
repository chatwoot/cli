package sdk

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIErrorCarriesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad", 1)
	for name, call := range map[string]func() error{
		"typed": func() error { _, err := client.Profile().Get(); return err },
		"raw": func() error {
			_, err := client.RequestRaw(http.MethodGet, "/api/v1/profile", nil, false, nil)
			return err
		},
	} {
		err := call()
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s: error = %v, want *APIError with 401", name, err)
		}
		if got := err.Error(); got != `API error 401: {"error":"Invalid token"}` {
			t.Fatalf("%s: message changed: %q", name, got)
		}
	}
}
