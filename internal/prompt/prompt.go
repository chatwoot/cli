// Package prompt provides small, dependency-light interactive terminal
// prompts (single-select, multi-select, free text, yes/no) built on the
// standard library plus golang.org/x/term. It is intentionally CLI-only so
// the import engine can stay non-interactive and provider-independent.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// Prompter is the interactive interface the CLI uses to gather choices. It is
// an interface so commands can be tested with a scripted implementation.
type Prompter interface {
	// SelectOne shows a numbered list and returns the chosen 0-based index.
	SelectOne(label string, options []string) (int, error)
	// SelectMany shows a numbered list and returns the chosen 0-based indices.
	// When allowAll is true the user may type "all" to select everything.
	SelectMany(label string, options []string, allowAll bool) ([]int, error)
	// Input reads a line of text, returning def when the input is empty.
	Input(label, def string) (string, error)
	// Confirm asks a yes/no question, defaulting to no on empty input.
	Confirm(label string) (bool, error)
}

// IsInteractive reports whether stdin is a terminal. Commands use this to
// require a TTY before prompting.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// TermPrompter is a Prompter backed by an io.Reader (input) and io.Writer
// (prompts/echo). Use NewTermPrompter(os.Stdin, os.Stdout) in production and
// inject strings.NewReader/bytes.Buffer in tests.
type TermPrompter struct {
	r *bufio.Reader
	w io.Writer
}

// NewTermPrompter builds a TermPrompter over the given reader and writer.
func NewTermPrompter(r io.Reader, w io.Writer) *TermPrompter {
	return &TermPrompter{r: bufio.NewReader(r), w: w}
}

func (p *TermPrompter) readLine() (string, error) {
	line, err := p.r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *TermPrompter) printOptions(label string, options []string) {
	fmt.Fprintln(p.w, label)
	for i, opt := range options {
		fmt.Fprintf(p.w, "  %d) %s\n", i+1, opt)
	}
}

// SelectOne shows a numbered list and reads a single choice, re-prompting on
// invalid input until a valid number is entered or the input ends (EOF).
func (p *TermPrompter) SelectOne(label string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("no options to choose from")
	}
	p.printOptions(label, options)
	for {
		fmt.Fprintf(p.w, "Enter number [1-%d]: ", len(options))
		line, err := p.readLine()
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintf(p.w, "Please enter a number between 1 and %d.\n", len(options))
	}
}

// SelectMany shows a numbered list and reads a comma-separated set of choices.
// "all" (when allowAll) selects everything. Re-prompts on invalid input.
func (p *TermPrompter) SelectMany(label string, options []string, allowAll bool) ([]int, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options to choose from")
	}
	p.printOptions(label, options)
	hint := "Enter numbers (comma-separated)"
	if allowAll {
		hint += " or 'all'"
	}
	for {
		fmt.Fprintf(p.w, "%s: ", hint)
		line, err := p.readLine()
		if err != nil {
			return nil, err
		}
		if allowAll && strings.EqualFold(line, "all") {
			idxs := make([]int, len(options))
			for i := range options {
				idxs[i] = i
			}
			return idxs, nil
		}
		idxs, ok := parseIndexList(line, len(options))
		if ok && len(idxs) > 0 {
			return idxs, nil
		}
		fmt.Fprintf(p.w, "Please enter one or more numbers between 1 and %d, separated by commas.\n", len(options))
	}
}

// parseIndexList parses "1, 3,2" into a deduplicated, ordered []int of 0-based
// indices. Returns ok=false if any token is not a valid in-range number.
func parseIndexList(line string, n int) ([]int, bool) {
	if strings.TrimSpace(line) == "" {
		return nil, false
	}
	seen := make(map[int]bool)
	var out []int
	for tok := range strings.SplitSeq(line, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		v, err := strconv.Atoi(tok)
		if err != nil || v < 1 || v > n {
			return nil, false
		}
		if !seen[v-1] {
			seen[v-1] = true
			out = append(out, v-1)
		}
	}
	return out, true
}

// Input reads a line, returning def on empty input.
func (p *TermPrompter) Input(label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(p.w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(p.w, "%s: ", label)
	}
	line, err := p.readLine()
	if err != nil {
		return "", err
	}
	if line == "" {
		return def, nil
	}
	return line, nil
}

// Confirm asks a yes/no question, defaulting to no on empty input.
func (p *TermPrompter) Confirm(label string) (bool, error) {
	fmt.Fprintf(p.w, "%s [y/N]: ", label)
	line, err := p.readLine()
	if err != nil {
		return false, err
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
