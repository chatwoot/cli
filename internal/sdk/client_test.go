package sdk

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
)

func TestVerboseAssignDoesNotLeakSensitiveUserTokens(t *testing.T) {
	const (
		accessSecret = "assign-access-secret"
		pubsubSecret = "assign-pubsub-secret"
	)

	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertJSONBody(t, r, map[string]any{
			"assignee_id": float64(7),
		})

		return contractResponse{body: sensitiveUserJSON(7, "Ada Lovelace", "ada@example.com", accessSecret, pubsubSecret)}
	})
	client.Verbose = true

	agentID := 7
	stderr := captureStderr(t, func() {
		got, err := client.Conversations().Assign(42, &agentID, 0)
		if err != nil {
			t.Fatalf("Assign returned error: %v", err)
		}
		if got.AccessToken != accessSecret || got.PubsubToken != pubsubSecret {
			t.Fatalf("unexpected token decode: %#v", got)
		}
	})

	assertDoesNotContainSensitiveValues(t, stderr, accessSecret, pubsubSecret)
}

func TestVerboseResponseLoggingDoesNotLeakNestedSensitiveTokens(t *testing.T) {
	const (
		accessSecret = "nested-access-secret"
		pubsubSecret = "nested-pubsub-secret"
		hmacSecret   = "nested-hmac-secret"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"payload": {
				"user": {
					"id": 7,
					"access_token": "`+accessSecret+`",
					"pubsub_token": "`+pubsubSecret+`",
					"hmac_identifier": "`+hmacSecret+`"
				}
			}
		}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()), WithVerbose(true))

	var decoded map[string]any
	stderr := captureStderr(t, func() {
		if err := client.Get("/users/7", nil, &decoded); err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	assertDoesNotContainSensitiveValues(t, stderr, accessSecret, pubsubSecret, hmacSecret)
}

func TestVerboseResponseLoggingRedactsSecretLikeFields(t *testing.T) {
	const (
		tokenSecret = "generic-token-secret"
		apiSecret   = "generic-api-secret"
		signingKey  = "generic-signing-key"
		hmacValue   = "generic-hmac-value"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"payload": {
				"session_token": "`+tokenSecret+`",
				"webhook_secret": "`+apiSecret+`",
				"signing_key": "`+signingKey+`",
				"hmac_digest": "`+hmacValue+`"
			}
		}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()), WithVerbose(true))

	var decoded map[string]any
	stderr := captureStderr(t, func() {
		if err := client.Get("/secrets", nil, &decoded); err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	assertDoesNotContainSensitiveValues(t, stderr, tokenSecret, apiSecret, signingKey, hmacValue)
}

func TestVerboseNonJSONResponseLoggingStripsTerminalControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "hello\x1b]52;c;Zm9v\aworld\x1b[31m")
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()), WithVerbose(true))

	var decoded map[string]any
	stderr := captureStderr(t, func() {
		if err := client.Get("/plain", nil, &decoded); err == nil {
			t.Fatal("expected JSON decode error")
		}
	})

	assertDoesNotContainSensitiveValues(t, stderr, "\x1b", "\a", "]52", "[31m")
	if !strings.Contains(stderr, "helloworld") {
		t.Fatalf("verbose output stripped printable content:\n%s", stderr)
	}
}

func TestAPIErrorBodyStripsTerminalControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad\x1b]52;c;Zm9v\aerror", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()))

	var decoded map[string]any
	err := client.Get("/boom", nil, &decoded)
	if err == nil {
		t.Fatal("expected API error")
	}
	got := err.Error()
	for _, disallowed := range []string{"\x1b", "\a", "]52"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("API error contained terminal control %q: %q", disallowed, got)
		}
	}
	if !strings.Contains(got, "baderror") {
		t.Fatalf("API error stripped printable content: %q", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = original
		_ = reader.Close()
		_ = writer.Close()
	})

	output := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		output <- buf.String()
	}()

	fn()

	_ = writer.Close()
	os.Stderr = original
	return <-output
}

func assertDoesNotContainSensitiveValues(t *testing.T, output string, values ...string) {
	t.Helper()

	for _, value := range values {
		if strings.Contains(output, value) {
			t.Fatalf("verbose output leaked sensitive value %q:\n%s", value, output)
		}
	}
}

func sensitiveUserJSON(id int, name, email, accessToken, pubsubToken string) string {
	body := userJSON(id, name, email)
	body = strings.Replace(body, `"access_token": "token"`, `"access_token": "`+accessToken+`"`, 1)
	body = strings.Replace(body, `"pubsub_token": "pubsub"`, `"pubsub_token": "`+pubsubToken+`"`, 1)
	return body
}
