package output

import (
	"bytes"
	"encoding/csv"
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
