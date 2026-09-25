package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintTableTextStripsTerminalControls(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter("text", false, false)
	p.Writer = &buf

	p.PrintTable(
		[]string{"ID", "Name", "Message"},
		[][]string{{
			"1",
			"Al\x1b[31mice\x1b[0m",
			"hello\r\nnext\tcol\x08!",
		}},
	)

	got := buf.String()
	for _, disallowed := range []string{"\x1b", "[31m", "[0m", "\r", "\b"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("text output contained unsafe terminal sequence %q:\n%s", disallowed, got)
		}
	}
	if !strings.Contains(got, "Alice") {
		t.Fatalf("text output stripped printable content:\n%s", got)
	}
	if !strings.Contains(got, "hellonextcol!") {
		t.Fatalf("text output did not preserve printable message content:\n%s", got)
	}
}

func TestPrintDetailTextStripsTerminalControls(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter("text", false, false)
	p.Writer = &buf

	p.PrintDetail([]KeyValue{
		{
			Key:   "Name",
			Value: "Eve\x1b]8;;https://example.test\aLink\x1b]8;;\a\nNext\x7f",
		},
	})

	got := buf.String()
	for _, disallowed := range []string{"\x1b", "\a", "\x7f", "https://example.test"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("detail output contained unsafe terminal sequence %q:\n%s", disallowed, got)
		}
	}
	if !strings.Contains(got, "EveLinkNext") {
		t.Fatalf("detail output did not preserve printable content:\n%s", got)
	}
}

func TestPrintTableCSVEscapesFormulaLikeCells(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter("csv", false, false)
	p.Writer = &buf

	p.PrintTable(
		[]string{"ID", "Name", "Email", "Phone", "Note", "Safe"},
		[][]string{{
			"1",
			"=cmd|' /C calc'!A0",
			"+15551234",
			"-10",
			"@SUM(A1:A2)",
			"plain",
		}},
	)

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("failed reading csv output: %v\n%s", err, buf.String())
	}
	if len(records) != 2 {
		t.Fatalf("expected header and one row, got %d records: %#v", len(records), records)
	}

	want := []string{
		"1",
		"'=cmd|' /C calc'!A0",
		"'+15551234",
		"'-10",
		"'@SUM(A1:A2)",
		"plain",
	}
	for i, cell := range want {
		if records[1][i] != cell {
			t.Fatalf("cell %d = %q, want %q\ncsv:\n%s", i, records[1][i], cell, buf.String())
		}
	}
}

func TestPrintTableJSONPreservesRawValues(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter("json", false, false)
	p.Writer = &buf

	raw := "=SUM(A1:A2)\x1b[31m"
	p.PrintTable([]string{"ID", "Value"}, [][]string{{"1", raw}})

	got := buf.String()
	if !strings.Contains(got, `=SUM(A1:A2)\u001b[31m`) {
		t.Fatalf("json output should preserve raw API values with JSON escaping:\n%s", got)
	}
}

func TestPrintDetailAlignsValues(t *testing.T) {
	var out bytes.Buffer
	p := NewPrinter("text", false, false)
	p.Writer = &out
	p.PrintDetail([]KeyValue{{Key: "ID", Value: "1"}, {Key: "Availability", Value: "online"}})

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || strings.Index(lines[0], "1") != strings.Index(lines[1], "online") {
		t.Fatalf("values not aligned:\n%s", out.String())
	}
}

func newTestPrinter(format string, quiet bool) (*Printer, *bytes.Buffer) {
	var buf bytes.Buffer
	p := NewPrinter(format, false, quiet)
	p.Writer = &buf
	return p, &buf
}

func TestPrintTableQuietPrintsFirstColumn(t *testing.T) {
	p, buf := newTestPrinter("json", true) // quiet wins over the format
	p.PrintTable([]string{"ID", "Name"}, [][]string{{"1", "Alice"}, {}, {"2\x1b[31m", "Bob"}})
	if got := buf.String(); got != "1\n2\n" {
		t.Fatalf("quiet output = %q, want IDs only", got)
	}
}

func TestPrintTableJSONMapsHeadersToCells(t *testing.T) {
	p, buf := newTestPrinter("json", false)
	p.PrintTable([]string{"ID", "Name", "Email"}, [][]string{{"1", "Alice"}})

	var got []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 1 || got[0]["ID"] != "1" || got[0]["Name"] != "Alice" {
		t.Fatalf("json rows = %#v", got)
	}
	if _, ok := got[0]["Email"]; ok {
		t.Fatalf("a missing cell must be omitted, got %#v", got[0])
	}
}

func TestPrintTableJSONEmptyIsArray(t *testing.T) {
	p, buf := newTestPrinter("json", false)
	p.PrintTable([]string{"ID"}, nil)
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("empty json table = %q, want []", buf.String())
	}
}

func TestPrintTableCSVWriteErrorGoesToStderr(t *testing.T) {
	p := NewPrinter("csv", false, false)
	p.Writer = failingWriter{}
	stderr := captureStderr(t, func() {
		p.PrintTable([]string{"ID"}, [][]string{{"1"}})
	})
	if !strings.Contains(stderr, "csv write failed") {
		t.Fatalf("stderr = %q, want a csv write failure", stderr)
	}
}

func TestPrintJSONEncodeErrorGoesToStderr(t *testing.T) {
	p, buf := newTestPrinter("json", false)
	stderr := captureStderr(t, func() {
		p.PrintJSON(map[string]any{"ch": make(chan int)})
	})
	if !strings.Contains(stderr, "json encode failed") {
		t.Fatalf("stderr = %q, want a json encode failure", stderr)
	}
	if buf.Len() != 0 {
		t.Fatalf("partial JSON written: %q", buf.String())
	}
}

func TestPrintDetailJSON(t *testing.T) {
	p, buf := newTestPrinter("json", false)
	p.PrintDetail([]KeyValue{{Key: "Name", Value: "Alice"}, {Key: "Role", Value: "agent"}})

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	if got["Name"] != "Alice" || got["Role"] != "agent" {
		t.Fatalf("json detail = %#v", got)
	}
}

func TestSanitizeTextSequences(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain text and unicode", "héllo wörld ✓", "héllo wörld ✓"},
		{"CSI color", "a\x1b[1;31mb\x1b[0mc", "abc"},
		{"unterminated CSI", "a\x1b[31", "a"},
		{"OSC ended by BEL", "a\x1b]0;title\x07b", "ab"},
		{"OSC ended by ST", "a\x1b]8;;https://x\x1b\\b", "ab"},
		{"unterminated OSC", "a\x1b]0;title", "a"},
		{"DCS string", "a\x1bPdata\x1b\\b", "ab"},
		{"two-byte escape", "a\x1bcb", "ab"},
		{"trailing ESC", "abc\x1b", "abc"},
		{"control characters", "a\x00b\x07c\x7fd\te", "abcde"},
		{"invalid UTF-8", "a\xffb", "ab"},
	}
	for _, tc := range cases {
		if got := SanitizeText(tc.in); got != tc.want {
			t.Errorf("%s: SanitizeText(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestEscapeCSVFormula(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"=SUM(1)": "'=SUM(1)",
		"+1":      "'+1",
		"-1":      "'-1",
		"@cmd":    "'@cmd",
		"plain":   "plain",
		"a=b":     "a=b",
	}
	for in, want := range cases {
		if got := escapeCSVFormula(in); got != want {
			t.Errorf("escapeCSVFormula(%q) = %q, want %q", in, got, want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = old
	_ = w.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}
