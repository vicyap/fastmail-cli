package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintJSON writes v as indented JSON to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table provides a simple tabular output writer.
type Table struct {
	writer *tabwriter.Writer
}

// NewTable creates a new table writer that writes to stdout.
func NewTable(out io.Writer) *Table {
	return &Table{
		writer: tabwriter.NewWriter(out, 0, 0, 2, ' ', 0),
	}
}

// Headers writes a header row.
func (t *Table) Headers(cols ...string) {
	fmt.Fprintln(t.writer, strings.Join(cols, "\t"))
}

// Row writes a data row.
func (t *Table) Row(cols ...string) {
	fmt.Fprintln(t.writer, strings.Join(cols, "\t"))
}

// Flush writes buffered output.
func (t *Table) Flush() error {
	return t.writer.Flush()
}
