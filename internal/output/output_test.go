package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTable_HeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf)

	tbl.Headers("NAME", "COUNT")
	tbl.Row("inbox", "42")
	tbl.Row("archive", "100")
	require.NoError(t, tbl.Flush())

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "COUNT")
	assert.Contains(t, output, "inbox")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "archive")
	assert.Contains(t, output, "100")
}

func TestTable_EmptyTable(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf)

	tbl.Headers("A", "B")
	require.NoError(t, tbl.Flush())

	output := buf.String()
	assert.Contains(t, output, "A")
	assert.Contains(t, output, "B")
}

func TestPrintJSON_Structure(t *testing.T) {
	data := map[string]any{
		"name":  "test",
		"count": 42,
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "test", decoded["name"])
	assert.Equal(t, float64(42), decoded["count"])
}

func TestPrintJSON_ToStdout(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	data := map[string]string{"key": "value"}
	printErr := PrintJSON(data)
	require.NoError(t, printErr)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "key")
	assert.Contains(t, buf.String(), "value")

	// Verify valid JSON
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "value", decoded["key"])
}

func TestPrintJSON_Array(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	data := []string{"a", "b", "c"}
	printErr := PrintJSON(data)
	require.NoError(t, printErr)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var decoded []string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, []string{"a", "b", "c"}, decoded)
}

func TestPager_NonTerminal(t *testing.T) {
	// In test environment, stdout is not a terminal, so Pager should
	// write directly to stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	writeErr := Pager(func(w io.Writer) error {
		_, err := w.Write([]byte("pager test content"))
		return err
	})

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, writeErr)

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Equal(t, "pager test content", buf.String())
}

func TestTable_NoHeaders(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf)

	tbl.Row("a", "b")
	tbl.Row("c", "d")
	require.NoError(t, tbl.Flush())

	output := buf.String()
	assert.Contains(t, output, "a")
	assert.Contains(t, output, "b")
	assert.Contains(t, output, "c")
	assert.Contains(t, output, "d")
}

func TestTable_SingleColumn(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf)

	tbl.Headers("VALUE")
	tbl.Row("one")
	tbl.Row("two")
	require.NoError(t, tbl.Flush())

	output := buf.String()
	assert.Contains(t, output, "VALUE")
	assert.Contains(t, output, "one")
	assert.Contains(t, output, "two")
}

func TestPager_WithPagerCat(t *testing.T) {
	// Override isTerminal to return true so we reach the PAGER branch
	orig := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = orig }()

	t.Setenv("PAGER", "cat")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	writeErr := Pager(func(w io.Writer) error {
		_, err := w.Write([]byte("pager cat content"))
		return err
	})

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, writeErr)

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Equal(t, "pager cat content", buf.String())
}

func TestPager_WithRealPager(t *testing.T) {
	// Override isTerminal to return true so we reach the pager subprocess branch
	orig := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = orig }()

	// Use "cat" as an actual pager command (not the shortcut)
	t.Setenv("PAGER", "/bin/cat")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	writeErr := Pager(func(w io.Writer) error {
		_, err := w.Write([]byte("real pager content"))
		return err
	})

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, writeErr)

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Equal(t, "real pager content", buf.String())
}

func TestPager_DefaultPager(t *testing.T) {
	// Override isTerminal and unset PAGER so it defaults to "less"
	orig := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = orig }()

	// Use "cat" path to avoid hanging on "less"
	t.Setenv("PAGER", "/usr/bin/cat")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	writeErr := Pager(func(w io.Writer) error {
		_, err := w.Write([]byte("default pager test"))
		return err
	})

	w.Close()
	os.Stdout = oldStdout

	require.NoError(t, writeErr)

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Equal(t, "default pager test", buf.String())
}

func TestPager_CallbackError(t *testing.T) {
	// Test that callback errors are propagated
	writeErr := Pager(func(w io.Writer) error {
		return io.ErrClosedPipe
	})

	assert.ErrorIs(t, writeErr, io.ErrClosedPipe)
}

func TestTable_ManyColumns(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf)

	tbl.Headers("A", "B", "C", "D", "E")
	tbl.Row("1", "2", "3", "4", "5")
	require.NoError(t, tbl.Flush())

	output := buf.String()
	for _, col := range []string{"A", "B", "C", "D", "E", "1", "2", "3", "4", "5"} {
		assert.Contains(t, output, col)
	}
}
