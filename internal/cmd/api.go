package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"

	"github.com/chatwoot/cli/internal/output"
)

type ApiCmd struct {
	Method string   `short:"X" placeholder:"METHOD" help:"HTTP method, like PATCH or DELETE. Defaults to POST when you send --data, otherwise GET."`
	Data   string   `short:"d" placeholder:"JSON|@FILE" help:"JSON to send. Use @file.json to send a file, or @- to read from stdin."`
	Header []string `short:"H" placeholder:"HEADER" help:"Extra header, like 'X-Trace: 1'. Repeat for more."`
	Exact  bool     `help:"Send the path as-is instead of under your account (for paths outside /api/v1/accounts)."`
	Path   string   `arg:"" help:"Where to send it, like /conversations/123."`
}

func (c *ApiCmd) Help() string {
	return `Paths go under your account, so /conversations/123 calls
/api/v1/accounts/<id>/conversations/123. The response is printed as JSON.

A request other than GET can change data, so check it before you run it.

Examples:
  chatwoot api /conversations/123
  chatwoot api -X PATCH /conversations/123 --data '{"status":"open"}'
  chatwoot api --exact /api/v1/profile
  chatwoot @staging api /inboxes

API reference: https://developers.chatwoot.com/api-reference/introduction`
}

func (c *ApiCmd) Run(app *App) error {
	method := strings.ToUpper(c.Method)
	if method == "" {
		if c.Data != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	path, accountScoped, err := normalizeAPIPath(c.Path, c.Exact)
	if err != nil {
		return err
	}

	body, err := apiRequestBody(c.Data)
	if err != nil {
		return err
	}

	headers, err := parseAPIHeaders(c.Header)
	if err != nil {
		return err
	}

	resp, err := app.Client.RequestRaw(method, path, body, accountScoped, headers)
	if err != nil {
		return err
	}

	printAPIResponse(app.Printer.Writer, resp.Body)
	return nil
}

func normalizeAPIPath(path string, exact bool) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, fmt.Errorf("path is required")
	}

	parsed, err := url.Parse(path)
	if err != nil {
		return "", false, fmt.Errorf("invalid path: %w", err)
	}
	if parsed.IsAbs() {
		return "", false, fmt.Errorf("absolute URLs are not supported; pass a path under the configured Chatwoot base URL")
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if exact || strings.HasPrefix(path, "/api/") {
		return path, false, nil
	}
	return path, true, nil
}

func apiRequestBody(data string) (io.Reader, error) {
	if data == "" {
		return nil, nil
	}
	if !strings.HasPrefix(data, "@") {
		return strings.NewReader(data), nil
	}
	if data == "@-" {
		return os.Stdin, nil
	}

	body, err := os.ReadFile(strings.TrimPrefix(data, "@"))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	return bytes.NewReader(body), nil
}

func parseAPIHeaders(headerArgs []string) (http.Header, error) {
	headers := make(http.Header)
	for _, arg := range headerArgs {
		name, value, ok := strings.Cut(arg, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid header %q; expected 'Name: value'", arg)
		}
		headers.Add(textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name)), strings.TrimSpace(value))
	}
	return headers, nil
}

func printAPIResponse(w io.Writer, body []byte) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return
	}

	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(decoded)
		return
	}

	safeBody := output.SanitizeText(string(body))
	_, _ = io.WriteString(w, safeBody)
	if !strings.HasSuffix(safeBody, "\n") {
		_, _ = fmt.Fprintln(w)
	}
}
