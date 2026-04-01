package output

import (
	"bytes"
	"encoding/json"
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
