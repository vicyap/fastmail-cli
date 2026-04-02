package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
)

// PrintJSON writes v as indented JSON to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

var headerStyle = color.New(color.Bold)

// Table provides a simple tabular output writer.
type Table struct {
	writer    *tabwriter.Writer
	hasHeader bool
}

// NewTable creates a new table writer that writes to the given writer.
func NewTable(out io.Writer) *Table {
	return &Table{
		writer: tabwriter.NewWriter(out, 0, 0, 2, ' ', 0),
	}
}

// Headers writes a bold header row.
func (t *Table) Headers(cols ...string) {
	t.hasHeader = true
	colored := make([]string, len(cols))
	for i, c := range cols {
		colored[i] = headerStyle.Sprint(c)
	}
	fmt.Fprintln(t.writer, strings.Join(colored, "\t"))
}

// Row writes a data row.
func (t *Table) Row(cols ...string) {
	fmt.Fprintln(t.writer, strings.Join(cols, "\t"))
}

// Flush writes buffered output.
func (t *Table) Flush() error {
	return t.writer.Flush()
}

// Pager pipes output through $PAGER (or "less") if stdout is a terminal.
// The callback writes to the pager's stdin. If not a terminal or PAGER is
// "cat", the callback writes directly to stdout.
func Pager(fn func(w io.Writer) error) error {
	if !isTerminal() {
		return fn(os.Stdout)
	}

	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	if pager == "cat" {
		return fn(os.Stdout)
	}

	cmd := exec.Command(pager)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fn(os.Stdout)
	}

	if err := cmd.Start(); err != nil {
		return fn(os.Stdout)
	}

	writeErr := fn(pipe)
	pipe.Close()
	cmdErr := cmd.Wait()

	if writeErr != nil {
		return writeErr
	}
	// Ignore pager exit errors (e.g., user quit with 'q')
	_ = cmdErr
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
