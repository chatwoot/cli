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
	Method string   `short:"X" placeholder:"METHOD" help:"HTTP method. Defaults to POST with --data, otherwise GET."`
	Data   string   `short:"d" placeholder:"JSON|@FILE" help:"JSON request body, or @file to read body from a file."`
	Header []string `short:"H" placeholder:"HEADER" help:"Additional request header, as 'Name: value'. Repeatable."`
	Exact  bool     `help:"Use the path exactly as provided under the configured base URL."`
	Path   string   `arg:"" help:"Endpoint path. /conversations/123 expands under the configured account."`
}

func (c *ApiCmd) Help() string {
	return `Account-relative paths such as /conversations/123 are expanded under
/api/v1/accounts/<account_id>. Use --exact for non-account-scoped paths.

The default method is GET, or POST when --data is provided. Override with -X.

Examples:
  chatwoot api /conversations/123
  chatwoot api -X PATCH /conversations/123 --data '{"status":"open"}'
  chatwoot api --exact /api/v1/profile`
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
