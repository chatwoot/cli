package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"
)

type Printer struct {
	Format  string
	Writer  io.Writer
	NoColor bool
	Quiet   bool
}

type KeyValue struct {
	Key   string
	Value string
}

func NewPrinter(format string, noColor, quiet bool) *Printer {
	return &Printer{
		Format:  format,
		Writer:  os.Stdout,
		NoColor: noColor,
		Quiet:   quiet,
	}
}

// PrintTable renders tabular data in the configured format.
// In quiet mode, only the first column (IDs) is printed.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	if p.Quiet {
		for _, row := range rows {
			if len(row) > 0 {
				_, _ = fmt.Fprintln(p.Writer, SanitizeText(row[0]))
			}
		}
		return
	}

	switch p.Format {
	case "json":
		p.tableAsJSON(headers, rows)
	case "csv":
		p.tableAsCSV(headers, rows)
	default:
		p.tableAsText(headers, rows)
	}
}

// PrintJSON outputs v as indented JSON. Used for full-fidelity API responses.
func (p *Printer) PrintJSON(v interface{}) {
	enc := json.NewEncoder(p.Writer)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "output: json encode failed:", err)
	}
}

// PrintDetail renders key-value pairs for a single record view.
func (p *Printer) PrintDetail(pairs []KeyValue) {
	if p.Format == "json" {
		m := make(map[string]string, len(pairs))
		for _, kv := range pairs {
			m[kv.Key] = kv.Value
		}
		p.PrintJSON(m)
		return
	}

	maxKey := 0
	for _, kv := range pairs {
		if len(kv.Key) > maxKey {
			maxKey = len(kv.Key)
		}
	}

	for _, kv := range pairs {
		_, _ = fmt.Fprintf(p.Writer, "%-*s  %s\n", maxKey, kv.Key+":", SanitizeText(kv.Value))
	}
}

func (p *Printer) tableAsText(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(p.Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(sanitizeTextRow(row), "\t"))
	}
	_ = w.Flush()
}

func (p *Printer) tableAsJSON(headers []string, rows [][]string) {
	result := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				m[h] = row[i]
			}
		}
		result = append(result, m)
	}
	p.PrintJSON(result)
}

func (p *Printer) tableAsCSV(headers []string, rows [][]string) {
	w := csv.NewWriter(p.Writer)
	// Errors are typically broken-pipe (downstream `| head`); surface only
	// the final flush error if any.
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(escapeCSVFormulaRow(row))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Fprintln(os.Stderr, "output: csv write failed:", err)
	}
}

func sanitizeTextRow(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = SanitizeText(cell)
	}
	return out
}

// SanitizeText removes terminal control sequences and other non-printable
// control characters from text before it is written to a terminal.
func SanitizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			i = skipANSISequence(s, i)
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
		i += size
	}

	return b.String()
}

func skipANSISequence(s string, i int) int {
	if i+1 >= len(s) {
		return i + 1
	}

	switch s[i+1] {
	case '[':
		// CSI sequence: ESC [ ... final-byte
		for j := i + 2; j < len(s); j++ {
			if s[j] >= 0x40 && s[j] <= 0x7e {
				return j + 1
			}
		}
		return len(s)
	case ']', 'P', '^', '_', 'X':
		// OSC and string-control sequences end with BEL or ST (ESC \).
		for j := i + 2; j < len(s); j++ {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
		}
		return len(s)
	default:
		return i + 2
	}
}

func escapeCSVFormulaRow(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = escapeCSVFormula(cell)
	}
	return out
}

func escapeCSVFormula(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	default:
		return s
	}
}
