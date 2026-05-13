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
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"payload": {
				"user": {
					"id": 7,
					"access_token": "`+accessSecret+`",
					"pubsub_token": "`+pubsubSecret+`"
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

	assertDoesNotContainSensitiveValues(t, stderr, accessSecret, pubsubSecret)
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
