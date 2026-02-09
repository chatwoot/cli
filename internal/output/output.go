package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
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
				fmt.Fprintln(p.Writer, row[0])
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
	enc.Encode(v)
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
		fmt.Fprintf(p.Writer, "%-*s  %s\n", maxKey, kv.Key+":", kv.Value)
	}
}

func (p *Printer) tableAsText(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(p.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
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
	w.Write(headers)
	for _, row := range rows {
		w.Write(row)
	}
	w.Flush()
}
