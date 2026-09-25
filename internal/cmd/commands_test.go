package cmd

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// cmdEnv is a fake Chatwoot API and an App wired to it, for running commands
// end to end without a real instance.
type cmdEnv struct {
	t        *testing.T
	app      *App
	requests []string                  // "METHOD /path?query", in order
	bodies   map[string]map[string]any // request body per "METHOD /path"
}

// newCmdEnv serves routes, keyed "METHOD /path" with the JSON body to return.
// Unknown routes answer 404, which fails the command under test.
func newCmdEnv(t *testing.T, routes map[string]string) *cmdEnv {
	t.Helper()
	isolateAuthEnv(t)
	env := &cmdEnv{t: t, bodies: map[string]map[string]any{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		env.requests = append(env.requests, key+"?"+r.URL.RawQuery)
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			var body map[string]any
			_ = json.Unmarshal(data, &body)
			env.bodies[key] = body
		}
		body, ok := routes[key]
		if !ok {
			http.Error(w, "unexpected "+key, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{Default: "test", Accounts: []config.Account{{Name: "test", BaseURL: server.URL, ID: 1, UserID: 7}}}
	env.app = &App{Client: sdk.NewClient(server.URL, "tok", 1), Config: cfg, Account: &cfg.Accounts[0]}
	return env
}

// run executes a command and returns everything it printed, whether through
// the Printer or straight to stdout.
func (e *cmdEnv) run(format string, quiet bool, run func(*App) error) (string, error) {
	e.t.Helper()
	f, err := os.CreateTemp(e.t.TempDir(), "out")
	if err != nil {
		e.t.Fatalf("CreateTemp: %v", err)
	}
	old := os.Stdout
	os.Stdout = f
	printer := output.NewPrinter(format, false, quiet)
	printer.Writer = f
	e.app.Printer = printer
	runErr := run(e.app)
	os.Stdout = old
	_ = f.Close()
	data, _ := os.ReadFile(f.Name())
	return string(data), runErr
}

func (e *cmdEnv) mustRun(format string, quiet bool, run func(*App) error) string {
	e.t.Helper()
	out, err := e.run(format, quiet, run)
	if err != nil {
		e.t.Fatalf("run: %v\noutput:\n%s", err, out)
	}
	return out
}

func assertContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

const (
	convFixture = `{"id":4521,"status":"open","priority":"high","messages_count":3,"created_at":1700000000,` +
		`"last_activity_at":1700003600,"labels":["billing","vip"],"meta":{"sender":{"id":88,"name":"Jane",` +
		`"email":"jane@example.com"},"assignee":{"id":7,"name":"Ada"},"team":{"id":2,"name":"Support"},` +
		`"channel":"Channel::WebWidget"}}`
	contactFixture = `{"id":88,"name":"Jane","email":"jane@example.com","phone_number":"+1555","company_name":"Acme",` +
		`"conversations_count":4,"created_at":1700000000}`
)

// Every list command renders a table, raw JSON, IDs in quiet mode, and a
// friendly line when there is nothing to show.
func TestListCommands(t *testing.T) {
	cases := []struct {
		name      string
		route     string
		body      string
		empty     string
		run       func(*App) error
		textWants []string
		emptyWant string
		quietWant string
	}{
		{
			name: "convs", route: "GET /api/v1/accounts/1/conversations",
			body:      `{"data":{"meta":{"all_count":1},"payload":[` + convFixture + `]}}`,
			empty:     `{"data":{"meta":{},"payload":[]}}`,
			run:       (&ConvsCmd{Status: "open", Assignee: "me", Page: 1}).Run,
			textWants: []string{"4521", "open", "Jane", "Ada", "Channel::WebWidget", "billing, vip"},
			emptyWant: "No conversations found.", quietWant: "4521\n",
		},
		{
			name: "conv messages", route: "GET /api/v1/accounts/1/conversations/4521/messages",
			body: `{"meta":{},"payload":[{"id":1,"content":"hello\nthere","message_type":0,"created_at":1700000000,` +
				`"sender":{"name":"Jane"}},{"id":2,"content":"internal","message_type":1,"private":true}]}`,
			empty:     `{"meta":{},"payload":[]}`,
			run:       (&ConvMessagesCmd{ID: 4521}).Run,
			textWants: []string{"incoming", "Jane", "hello there", "note", "internal"},
			emptyWant: "No messages found.", quietWant: "1\n2\n",
		},
		{
			name: "contacts", route: "GET /api/v1/accounts/1/contacts",
			body:      `{"meta":{"current_page":"1"},"payload":[` + contactFixture + `]}`,
			empty:     `{"meta":{},"payload":[]}`,
			run:       (&ContactsCmd{Page: 1}).Run,
			textWants: []string{"88", "Jane", "jane@example.com", "+1555"},
			emptyWant: "No contacts found.", quietWant: "88\n",
		},
		{
			name: "contacts search", route: "GET /api/v1/accounts/1/contacts/search",
			body:      `{"meta":{},"payload":[` + contactFixture + `]}`,
			empty:     `{"meta":{},"payload":[]}`,
			run:       (&ContactsCmd{Search: "jane", Page: 1, Sort: "name"}).Run,
			textWants: []string{"Jane"},
			emptyWant: "No contacts found.", quietWant: "88\n",
		},
		{
			name: "contact conversations", route: "GET /api/v1/accounts/1/contacts/88/conversations",
			body:      `{"payload":[` + convFixture + `]}`,
			empty:     `{"payload":[]}`,
			run:       (&ContactConversationsCmd{ID: 88}).Run,
			textWants: []string{"4521", "open", "Ada"},
			emptyWant: "No conversations found for this contact.", quietWant: "4521\n",
		},
		{
			name: "inboxes", route: "GET /api/v1/accounts/1/inboxes",
			body:      `{"payload":[{"id":5,"name":"Website","channel_type":"Channel::WebWidget"}]}`,
			empty:     `{"payload":[]}`,
			run:       (&InboxesCmd{}).Run,
			textWants: []string{"5", "Website", "Channel::WebWidget"},
			emptyWant: "No inboxes found.", quietWant: "5\n",
		},
		{
			name: "agents", route: "GET /api/v1/accounts/1/agents",
			body:      `[{"id":7,"name":"Ada Lovelace","email":"ada@example.com","availability_status":"online","role":"agent"}]`,
			empty:     `[]`,
			run:       (&AgentsCmd{}).Run,
			textWants: []string{"Ada Lovelace", "ada@example.com", "online", "agent"},
			emptyWant: "No agents found.", quietWant: "7\n",
		},
		{
			name: "labels", route: "GET /api/v1/accounts/1/labels",
			body:      `{"payload":[{"id":1,"title":"billing","color":"#f00","description":"Money"}]}`,
			empty:     `{"payload":[]}`,
			run:       (&LabelsCmd{}).Run,
			textWants: []string{"billing", "#f00", "Money"},
			emptyWant: "No labels found.", quietWant: "1\n",
		},
		{
			name: "teams", route: "GET /api/v1/accounts/1/teams",
			body:      `[{"id":2,"name":"Support","description":"Tier 1"}]`,
			empty:     `[]`,
			run:       (&TeamsCmd{}).Run,
			textWants: []string{"Support", "Tier 1"},
			emptyWant: "No teams found.", quietWant: "2\n",
		},
		{
			name: "help centers", route: "GET /api/v1/accounts/1/portals",
			body:      `{"payload":[{"id":1,"name":"Docs","slug":"docs","meta":{"all_articles_count":9,"default_locale":"fr"}}]}`,
			empty:     `{"payload":[]}`,
			run:       (&HCsCmd{}).Run,
			textWants: []string{"Docs", "docs", "fr", "9"},
			emptyWant: "No help centers found.", quietWant: "1\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newCmdEnv(t, map[string]string{tc.route: tc.body})
			assertContains(t, env.mustRun("text", false, tc.run), tc.textWants...)

			jsonOut := env.mustRun("json", false, tc.run)
			if !json.Valid([]byte(jsonOut)) {
				t.Fatalf("-o json output is not JSON:\n%s", jsonOut)
			}

			if got := env.mustRun("text", true, tc.run); got != tc.quietWant {
				t.Fatalf("quiet output = %q, want %q", got, tc.quietWant)
			}

			empty := newCmdEnv(t, map[string]string{tc.route: tc.empty})
			assertContains(t, empty.mustRun("text", false, tc.run), tc.emptyWant)

			failing := newCmdEnv(t, nil)
			if _, err := failing.run("text", false, tc.run); err == nil {
				t.Fatal("an API error was not returned")
			}
		})
	}
}

func TestViewCommands(t *testing.T) {
	cases := []struct {
		name  string
		route string
		body  string
		run   func(*App) error
		wants []string
	}{
		{
			name: "conv view", route: "GET /api/v1/accounts/1/conversations/4521", body: convFixture,
			run:   (&ConvViewCmd{ID: 4521}).Run,
			wants: []string{"4521", "open", "high", "Jane <jane@example.com>", "Ada", "Support", "billing, vip", "Messages:"},
		},
		{
			name: "conv view without optional fields", route: "GET /api/v1/accounts/1/conversations/9",
			body:  `{"id":9,"status":"pending","meta":{}}`,
			run:   (&ConvViewCmd{ID: 9}).Run,
			wants: []string{"pending", "Priority:", "none"},
		},
		{
			name: "contact view", route: "GET /api/v1/accounts/1/contacts/88", body: `{"payload":` + contactFixture + `}`,
			run:   (&ContactViewCmd{ID: 88}).Run,
			wants: []string{"Jane", "jane@example.com", "+1555", "Acme", "Conversations:", "4"},
		},
		{
			name: "inbox view", route: "GET /api/v1/accounts/1/inboxes/5",
			body:  `{"id":5,"name":"Website","channel_type":"Channel::WebWidget","greeting_message":"Hi there"}`,
			run:   (&InboxViewCmd{ID: 5}).Run,
			wants: []string{"Website", "Channel::WebWidget", "Hi there"},
		},
		{
			name: "help center article", route: "GET /hc/docs/articles/setup.json",
			body:  `{"id":3,"title":"Setup","slug":"setup","category_id":12,"views":40,"content":"  Step one  "}`,
			run:   (&HCArticleCmd{PortalSlug: "docs", ArticleSlug: "setup"}).Run,
			wants: []string{"Setup", "setup", "12", "40", "Step one"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newCmdEnv(t, map[string]string{tc.route: tc.body})
			assertContains(t, env.mustRun("text", false, tc.run), tc.wants...)
			if out := env.mustRun("json", false, tc.run); !json.Valid([]byte(out)) {
				t.Fatalf("-o json output is not JSON:\n%s", out)
			}
			if _, err := newCmdEnv(t, nil).run("text", false, tc.run); err == nil {
				t.Fatal("an API error was not returned")
			}
		})
	}
}

func TestConvReply(t *testing.T) {
	const route = "POST /api/v1/accounts/1/conversations/4521/messages"
	env := newCmdEnv(t, map[string]string{route: `{"id":99,"content":"hi"}`})

	assertContains(t, env.mustRun("text", false, (&ConvReplyCmd{ID: 4521, Text: "hi"}).Run),
		"Sent reply on conversation 4521 (message 99).")
	if body := env.bodies[route]; body["content"] != "hi" || body["private"] == true || body["message_type"] != "outgoing" {
		t.Fatalf("reply body = %#v", body)
	}

	assertContains(t, env.mustRun("text", false, (&ConvReplyCmd{ID: 4521, Text: "fyi", Private: true}).Run), "Sent note")
	if env.bodies[route]["private"] != true {
		t.Fatalf("private note body = %#v", env.bodies[route])
	}

	if got := env.mustRun("text", true, (&ConvReplyCmd{ID: 4521, Text: "hi"}).Run); got != "99\n" {
		t.Fatalf("quiet reply = %q, want the message ID", got)
	}
	if _, err := newCmdEnv(t, nil).run("text", false, (&ConvReplyCmd{ID: 4521, Text: "hi"}).Run); err == nil {
		t.Fatal("a failed reply returned no error")
	}
}

func TestConvStatusOutput(t *testing.T) {
	const route = "POST /api/v1/accounts/1/conversations/4521/toggle_status"
	env := newCmdEnv(t, map[string]string{route: `{"payload":{"success":true}}`})

	assertContains(t, env.mustRun("text", false, (&ConvSnoozeCmd{ID: 4521, Until: "2d"}).Run), "Conversation 4521 -> snoozed (until ")
	if env.bodies[route]["snoozed_until"] == nil {
		t.Fatalf("snooze body = %#v, want snoozed_until", env.bodies[route])
	}
	if got := env.mustRun("text", true, (&ConvResolveCmd{ID: 4521}).Run); got != "4521\n" {
		t.Fatalf("quiet resolve = %q", got)
	}
	if _, err := env.run("text", false, (&ConvSnoozeCmd{ID: 4521, Until: "someday"}).Run); err == nil ||
		!strings.Contains(err.Error(), "invalid --until") {
		t.Fatalf("bad --until error = %v", err)
	}
	if _, err := newCmdEnv(t, nil).run("text", false, (&ConvOpenCmd{ID: 4521}).Run); err == nil {
		t.Fatal("a failed status change returned no error")
	}
}

func TestConvAssign(t *testing.T) {
	routes := map[string]string{
		"POST /api/v1/accounts/1/conversations/4521/assignments": `{"id":7}`,
		"GET /api/v1/accounts/1/agents": `[{"id":7,"name":"Ada Lovelace","email":"ada@example.com"},` +
			`{"id":8,"name":"Adam Smith","email":"adam@example.com"},{"id":9,"name":"Grace","email":"grace@example.com"}]`,
	}
	const assign = "POST /api/v1/accounts/1/conversations/4521/assignments"

	cases := []struct {
		name      string
		cmd       ConvAssignCmd
		wantOut   string
		wantAgent any
		wantTeam  any
	}{
		{"agent id", ConvAssignCmd{ID: 4521, Agent: "42"}, "assigned to agent 42.", float64(42), nil},
		{"agent and team", ConvAssignCmd{ID: 4521, Agent: "42", Team: 3}, "assigned to agent 42, team 3.", float64(42), float64(3)},
		{"team only", ConvAssignCmd{ID: 4521, Team: 3}, "assigned to team 3.", nil, float64(3)},
		{"name substring", ConvAssignCmd{ID: 4521, Agent: "grace"}, "assigned to agent 9.", float64(9), nil},
		{"email prefix", ConvAssignCmd{ID: 4521, Agent: "adam@"}, "assigned to agent 8.", float64(8), nil},
		{"me from cached user", ConvAssignCmd{ID: 4521, Agent: "me"}, "assigned to agent 7.", float64(7), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newCmdEnv(t, routes)
			cmd := tc.cmd
			assertContains(t, env.mustRun("text", false, cmd.Run), tc.wantOut)
			body := env.bodies[assign]
			if body["assignee_id"] != tc.wantAgent || body["team_id"] != tc.wantTeam {
				t.Fatalf("assign body = %#v, want assignee %v team %v", body, tc.wantAgent, tc.wantTeam)
			}
		})
	}

	env := newCmdEnv(t, routes)
	if got := env.mustRun("text", true, (&ConvAssignCmd{ID: 4521, Team: 3}).Run); got != "4521\n" {
		t.Fatalf("quiet assign = %q", got)
	}
	for name, cmd := range map[string]ConvAssignCmd{
		"no target":      {ID: 4521},
		"ambiguous name": {ID: 4521, Agent: "ada"},
		"unknown name":   {ID: 4521, Agent: "nobody"},
		"blank agent":    {ID: 4521, Agent: "  "},
	} {
		cmd := cmd
		if _, err := env.run("text", false, cmd.Run); err == nil {
			t.Errorf("%s: assign succeeded, want an error", name)
		}
	}
	if _, err := newCmdEnv(t, nil).run("text", false, (&ConvAssignCmd{ID: 4521, Agent: "grace"}).Run); err == nil ||
		!strings.Contains(err.Error(), "agent lookup failed") {
		t.Fatalf("agent lookup failure = %v", err)
	}
}

func TestConvAssignMeFetchesProfileOnce(t *testing.T) {
	env := newCmdEnv(t, map[string]string{
		"GET /api/v1/profile": `{"id":55,"name":"Me"}`,
		"POST /api/v1/accounts/1/conversations/4521/assignments": `{"id":55}`,
	})
	env.app.Account.UserID = 0

	env.mustRun("text", false, (&ConvAssignCmd{ID: 4521, Agent: "me"}).Run)
	env.mustRun("text", false, (&ConvAssignCmd{ID: 4521, Agent: "me"}).Run)
	profileCalls := 0
	for _, r := range env.requests {
		if strings.HasPrefix(r, "GET /api/v1/profile") {
			profileCalls++
		}
	}
	if profileCalls != 1 || env.app.Account.UserID != 55 {
		t.Fatalf("profile fetched %d times, cached user %d; want once and 55", profileCalls, env.app.Account.UserID)
	}

	failing := newCmdEnv(t, nil)
	failing.app.Account.UserID = 0
	if _, err := failing.run("text", false, (&ConvAssignCmd{ID: 4521, Agent: "me"}).Run); err == nil ||
		!strings.Contains(err.Error(), "cannot resolve 'me'") {
		t.Fatalf("profile failure = %v", err)
	}
}

func TestConvUnassignLabelPriority(t *testing.T) {
	routes := map[string]string{
		"POST /api/v1/accounts/1/conversations/4521/assignments":     `{}`,
		"POST /api/v1/accounts/1/conversations/4521/labels":          `{"payload":["a","b","c"]}`,
		"POST /api/v1/accounts/1/conversations/4521/toggle_priority": `{}`,
	}
	env := newCmdEnv(t, routes)

	assertContains(t, env.mustRun("text", false, (&ConvUnassignCmd{ID: 4521}).Run), "Conversation 4521 unassigned.")

	out := env.mustRun("text", false, (&ConvLabelCmd{ID: 4521, Labels: []string{"a, b", "", "c"}}).Run)
	assertContains(t, out, "Conversation 4521 labels: a, b, c")
	if labels := env.bodies["POST /api/v1/accounts/1/conversations/4521/labels"]["labels"]; len(labels.([]any)) != 3 {
		t.Fatalf("labels body = %#v, want the flattened set", labels)
	}

	assertContains(t, env.mustRun("text", false, (&ConvPriorityCmd{ID: 4521, Level: "urgent"}).Run), "priority -> urgent.")
	env.mustRun("text", false, (&ConvPriorityCmd{ID: 4521, Level: "none"}).Run)
	if body := env.bodies["POST /api/v1/accounts/1/conversations/4521/toggle_priority"]; body["priority"] != nil {
		t.Fatalf("priority none body = %#v, want null", body)
	}

	for name, run := range map[string]func(*App) error{
		"unassign": (&ConvUnassignCmd{ID: 4521}).Run,
		"label":    (&ConvLabelCmd{ID: 4521, Labels: []string{"a"}}).Run,
		"priority": (&ConvPriorityCmd{ID: 4521, Level: "low"}).Run,
	} {
		if got := env.mustRun("text", true, run); got != "4521\n" {
			t.Errorf("quiet %s = %q", name, got)
		}
		if _, err := newCmdEnv(t, nil).run("text", false, run); err == nil {
			t.Errorf("%s: API failure returned no error", name)
		}
	}
}

func TestVersion(t *testing.T) {
	env := newCmdEnv(t, nil)
	env.app.Version = "1.2.3"
	if got := env.mustRun("text", false, (&VersionCmd{}).Run); got != "1.2.3\n" {
		t.Fatalf("version = %q", got)
	}
	env.app.Version = ""
	if got := env.mustRun("text", false, (&VersionCmd{}).Run); got != "dev\n" {
		t.Fatalf("empty version = %q, want dev", got)
	}

	release := func(status int, body string) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Errorf("Accept = %q", r.Header.Get("Accept"))
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}))
		t.Cleanup(server.Close)
		latestReleaseURL = server.URL
	}
	t.Cleanup(func() { latestReleaseURL = "https://api.github.com/repos/chatwoot/cli/releases/latest" })

	release(http.StatusOK, `{"tag_name":"v1.2.3"}`)
	for version, want := range map[string]string{
		"1.2.3":  "Up to date.",
		"v1.2.0": "Update available.",
		"":       "Running a dev build.",
	} {
		env.app.Version = version
		out := env.mustRun("text", false, (&VersionCmd{Check: true}).Run)
		assertContains(t, out, want, "Latest: v1.2.3")
	}
	env.app.Version = "1.2.0"
	assertContains(t, env.mustRun("text", false, (&VersionCmd{Check: true}).Run), "chatwoot v1.2.0", "install-cli")

	for name, setup := range map[string]func(){
		"non-200":     func() { release(http.StatusForbidden, `rate limited`) },
		"missing tag": func() { release(http.StatusOK, `{}`) },
		"bad json":    func() { release(http.StatusOK, `{`) },
		"bad url":     func() { latestReleaseURL = "http://[::1" },
		"unreachable": func() { release(http.StatusOK, ``); latestReleaseURL = "http://127.0.0.1:1" },
	} {
		setup()
		if _, err := env.run("text", false, (&VersionCmd{Check: true}).Run); err == nil || !strings.Contains(err.Error(), "check failed") {
			t.Errorf("%s: error = %v, want check failed", name, err)
		}
	}
}

func TestApiRequestBodySources(t *testing.T) {
	if body, err := apiRequestBody(""); body != nil || err != nil {
		t.Fatalf("empty data = (%v, %v), want no body", body, err)
	}

	file := t.TempDir() + "/body.json"
	if err := os.WriteFile(file, []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	body, err := apiRequestBody("@" + file)
	if err != nil {
		t.Fatalf("@file: %v", err)
	}
	if data, _ := io.ReadAll(body); string(data) != `{"from":"file"}` {
		t.Fatalf("@file body = %q", data)
	}

	if _, err := apiRequestBody("@" + t.TempDir() + "/missing.json"); err == nil || !strings.Contains(err.Error(), "read request body") {
		t.Fatalf("missing file error = %v", err)
	}

	r, w, _ := os.Pipe()
	_, _ = io.WriteString(w, `{"from":"stdin"}`)
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	body, err = apiRequestBody("@-")
	if err != nil {
		t.Fatalf("@-: %v", err)
	}
	if data, _ := io.ReadAll(body); string(data) != `{"from":"stdin"}` {
		t.Fatalf("@- body = %q", data)
	}
}

func TestApiCmdErrors(t *testing.T) {
	if !strings.Contains((&ApiCmd{}).Help(), "--exact") {
		t.Fatal("api help should explain --exact")
	}
	env := newCmdEnv(t, nil)
	for name, cmd := range map[string]ApiCmd{
		"empty path":   {Path: " "},
		"bad header":   {Path: "/x", Header: []string{"no-colon"}},
		"missing file": {Path: "/x", Data: "@/does/not/exist"},
		"api error":    {Path: "/conversations/1"},
	} {
		cmd := cmd
		if _, err := env.run("text", false, cmd.Run); err == nil {
			t.Errorf("%s: api succeeded, want an error", name)
		}
	}
}

func TestConfigPath(t *testing.T) {
	env := newCmdEnv(t, nil)
	want, _ := config.ConfigPath()
	if got := env.mustRun("text", false, (&ConfigPathCmd{}).Run); strings.TrimSpace(got) != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}
}

func TestAuthLogoutEverything(t *testing.T) {
	env := newCmdEnv(t, nil)
	saveTwoLogins(t)
	path, _ := config.ConfigPath()

	assertContains(t, env.mustRun("text", false, (&AuthLogoutCmd{}).Run), "Logged out successfully.")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config not removed: %v", err)
	}

	t.Setenv(config.APIKeyEnv, "env-token")
	assertContains(t, env.mustRun("text", false, (&AuthLogoutCmd{}).Run), "Not logged in.", config.APIKeyEnv+" is set")

	saveTwoLogins(t)
	assertContains(t, env.mustRun("text", false, (&AuthLogoutCmd{}).Run), "Logged out successfully.", config.APIKeyEnv+" is set")
}

func TestAuthLogoutInstanceEdgeCases(t *testing.T) {
	env := newCmdEnv(t, nil)
	if _, err := env.run("text", false, (&AuthLogoutCmd{URL: "ftp://x"}).Run); err == nil {
		t.Fatal("logout with an invalid URL succeeded")
	}
	assertContains(t, env.mustRun("text", false, (&AuthLogoutCmd{URL: "nowhere.example"}).Run), "Not logged in to nowhere.example.")

	cfg := saveTwoLogins(t)
	_ = cfg
	out := env.mustRun("text", false, (&AuthLogoutCmd{URL: "app.chatwoot.com"}).Run)
	assertContains(t, out, "Logged out of app.chatwoot.com (removed 2 accounts).", "No default account now")
}

func TestChooseDefault(t *testing.T) {
	accounts := []*config.Account{{Name: "acme", ID: 1}, {Name: "acme-staging", ID: 2}, {Name: "globex", ID: 3}}
	choose := func(input string, link int) string {
		t.Helper()
		var picked *config.Account
		_, _ = newCmdEnv(t, nil).run("text", false, func(*App) error {
			picked = chooseDefault(bufioReader(input), accounts, link)
			return nil
		})
		return picked.Name
	}

	cases := []struct {
		input string
		link  int
		want  string
	}{
		{"", 0, "acme"},                 // Enter / EOF takes the first
		{"globex\n", 0, "globex"},       // exact
		{"@glo\n", 0, "globex"},         // unique prefix, @ allowed
		{"acme\n", 0, "acme"},           // exact beats a longer prefix match
		{"acme-\n", 0, "acme-staging"},  // unique prefix
		{"nope\nglobex\n", 0, "globex"}, // retries after a bad answer
		{"nope", 0, "acme"},             // bad answer then EOF falls back
		{"", 3, "globex"},               // a pasted link's account wins
	}
	for _, tc := range cases {
		if got := choose(tc.input, tc.link); got != tc.want {
			t.Errorf("chooseDefault(%q, link %d) = %s, want %s", tc.input, tc.link, got, tc.want)
		}
	}
}

func TestAccessibleAccountsHint(t *testing.T) {
	if got := accessibleAccountsHint([]sdk.ProfileAccount{{ID: 7, Name: "Acme"}}); got != "this key can access account 7 (Acme)" {
		t.Fatalf("single account hint = %q", got)
	}
	if got := accessibleAccountsHint([]sdk.ProfileAccount{{ID: 7}, {ID: 9, Name: "Beta"}}); got != "this key can access accounts: 7, 9 (Beta)" {
		t.Fatalf("multi account hint = %q", got)
	}
}

func TestHelpCenterFieldFallbacks(t *testing.T) {
	portal := sdk.HelpCenterPortal{}
	portal.Meta.DefaultLocale = "fr"
	portal.Meta.AllArticlesCount = 9
	if portalDefaultLocale(portal) != "fr" || portalArticlesCount(portal) != 9 {
		t.Fatalf("portal fallbacks = %q, %d", portalDefaultLocale(portal), portalArticlesCount(portal))
	}
	portal.Config.DefaultLocale = "en"
	portal.Meta.PublishedCount = 4
	if portalDefaultLocale(portal) != "en" || portalArticlesCount(portal) != 4 {
		t.Fatalf("portal preferred fields = %q, %d", portalDefaultLocale(portal), portalArticlesCount(portal))
	}

	if got := articleCategory(sdk.HelpCenterArticle{CategoryID: 12}); got != "12" {
		t.Fatalf("category from id = %q", got)
	}
	if got := articleCategory(sdk.HelpCenterArticle{}); got != "" {
		t.Fatalf("no category = %q", got)
	}
	if got := articleSnippet(sdk.HelpCenterArticle{Content: "  body text  "}); got != "body text" {
		t.Fatalf("snippet from content = %q", got)
	}
}

func TestAccountsCommandsWithoutConfig(t *testing.T) {
	env := newCmdEnv(t, nil)
	if _, err := env.run("text", false, (&AccountsRenameCmd{Old: "a", New: "b"}).Run); err == nil {
		t.Fatal("rename without a config succeeded")
	}
	if _, err := env.run("text", false, (&UseCmd{Name: "a"}).Run); err == nil {
		t.Fatal("use without a config succeeded")
	}
	if _, err := env.run("text", false, (&AccountsListCmd{Refresh: true}).Run); err != nil {
		t.Fatalf("refresh without accounts should explain, not fail: %v", err)
	}

	saveTwoLogins(t)
	t.Setenv(config.APIKeyEnv, "env-token")
	if _, err := env.run("text", false, (&AccountsListCmd{Refresh: true}).Run); err == nil ||
		!strings.Contains(err.Error(), config.APIKeyEnv) {
		t.Fatalf("refresh with an env token = %v, want an explanation", err)
	}
	if out := env.mustRun("csv", false, (&AccountsListCmd{}).Run); !strings.Contains(out, "Name,Base URL,ID,User,Default") {
		t.Fatalf("csv accounts = %q", out)
	}
}

func TestUseWithoutDefault(t *testing.T) {
	env := newCmdEnv(t, nil)
	cfg := saveTwoLogins(t)
	cfg.Default = ""
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	assertContains(t, env.mustRun("text", false, (&UseCmd{}).Run), "No default account")
}

func bufioReader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestHCDefaultEdgeCases(t *testing.T) {
	const portals = "GET /api/v1/accounts/1/portals"
	env := newCmdEnv(t, map[string]string{portals: `{"payload":[{"id":1,"name":"Docs","slug":"docs"}]}`})

	assertContains(t, env.mustRun("text", false, (&HCDefaultCmd{}).Run), "No default help center set.")

	if _, err := env.run("text", false, (&HCDefaultCmd{Slug: "missing"}).Run); err == nil ||
		!strings.Contains(err.Error(), `no help center matched "missing"`) {
		t.Fatalf("unknown portal = %v", err)
	}
	if _, err := findHelpCenterPortal(env.app, " "); err == nil {
		t.Fatal("findHelpCenterPortal accepted a blank slug")
	}
	if _, err := newCmdEnv(t, nil).run("text", false, (&HCDefaultCmd{Slug: "docs"}).Run); err == nil {
		t.Fatal("a portals API failure returned no error")
	}

	// A portal without a default locale is saved without one.
	assertContains(t, env.mustRun("text", false, (&HCDefaultCmd{Slug: "docs"}).Run), "Default help center set to docs\n")
	if _, err := env.run("text", false, (&HCArticlesCmd{}).Run); err == nil ||
		!strings.Contains(err.Error(), "no default help center locale") {
		t.Fatalf("articles without a saved locale = %v", err)
	}

	// An ad-hoc `-a <id>` account isn't in the config, so it can't hold defaults.
	env.app.Account = &config.Account{BaseURL: env.app.Account.BaseURL, ID: 99}
	for name, run := range map[string]func(*App) error{
		"set":   (&HCDefaultCmd{Slug: "docs"}).Run,
		"clear": (&HCDefaultCmd{Clear: true}).Run,
	} {
		if _, err := env.run("text", false, run); err != errUnregisteredAccount {
			t.Errorf("%s on an unregistered account = %v, want errUnregisteredAccount", name, err)
		}
	}
}

func TestConvContactAPIErrors(t *testing.T) {
	if _, err := newCmdEnv(t, nil).run("text", false, (&ConvContactCmd{ID: 1}).Run); err == nil {
		t.Fatal("conversation lookup failure returned no error")
	}
	env := newCmdEnv(t, map[string]string{"GET /api/v1/accounts/1/conversations/1": `{"id":1,"meta":{"sender":{"id":88}}}`})
	if _, err := env.run("text", false, (&ConvContactCmd{ID: 1}).Run); err == nil {
		t.Fatal("contact lookup failure returned no error")
	}
}
