package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestRunAuthStatus(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := runAuthStatus(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), jmaptest.TestUsername)
}

func TestRunAuthStatus_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runAuthStatus(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), jmaptest.TestUsername)
	assert.Contains(t, buf.String(), jmaptest.TestAccountID)
}
