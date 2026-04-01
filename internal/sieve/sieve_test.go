package sieve

import (
	"encoding/json"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSieveScript_JSONRoundTrip(t *testing.T) {
	name := "my-filter"
	s := &SieveScript{
		ID:       "sieve-1",
		Name:     &name,
		BlobID:   "blob-123",
		IsActive: true,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var decoded SieveScript
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, jmap.ID("sieve-1"), decoded.ID)
	require.NotNil(t, decoded.Name)
	assert.Equal(t, "my-filter", *decoded.Name)
	assert.Equal(t, jmap.ID("blob-123"), decoded.BlobID)
	assert.True(t, decoded.IsActive)
}

func TestGet_MethodName(t *testing.T) {
	g := &Get{}
	assert.Equal(t, "SieveScript/get", g.Name())
	assert.Equal(t, []jmap.URI{URI}, g.Requires())
}

func TestSet_MethodName(t *testing.T) {
	s := &Set{}
	assert.Equal(t, "SieveScript/set", s.Name())
	assert.Equal(t, []jmap.URI{URI}, s.Requires())
}

func TestQuery_MethodName(t *testing.T) {
	q := &Query{}
	assert.Equal(t, "SieveScript/query", q.Name())
	assert.Equal(t, []jmap.URI{URI}, q.Requires())
}

func TestValidate_MethodName(t *testing.T) {
	v := &Validate{}
	assert.Equal(t, "SieveScript/validate", v.Name())
	assert.Equal(t, []jmap.URI{URI}, v.Requires())
}

func TestCapability_URI(t *testing.T) {
	cap := &Capability{}
	assert.Equal(t, jmap.URI("urn:ietf:params:jmap:sieve"), cap.URI())

	newCap := cap.New()
	assert.IsType(t, &Capability{}, newCap)
}

func TestSet_ActivateSerialization(t *testing.T) {
	scriptID := jmap.ID("sieve-1")
	s := &Set{
		Account:                 "u12345",
		OnSuccessActivateScript: &scriptID,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, "sieve-1", raw["onSuccessActivateScript"])
}

func TestSet_DeactivateSerialization(t *testing.T) {
	deactivate := true
	s := &Set{
		Account:                   "u12345",
		OnSuccessDeactivateScript: &deactivate,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, true, raw["onSuccessDeactivateScript"])
}

func TestGetResponse_Deserialization(t *testing.T) {
	jsonData := `{
		"accountId": "u12345",
		"state": "abc",
		"list": [
			{
				"id": "sieve-1",
				"name": "main-filter",
				"blobId": "blob-1",
				"isActive": true
			},
			{
				"id": "sieve-2",
				"name": "backup-filter",
				"blobId": "blob-2",
				"isActive": false
			}
		]
	}`

	var resp GetResponse
	require.NoError(t, json.Unmarshal([]byte(jsonData), &resp))

	assert.Equal(t, jmap.ID("u12345"), resp.Account)
	require.Len(t, resp.List, 2)
	assert.Equal(t, jmap.ID("sieve-1"), resp.List[0].ID)
	assert.True(t, resp.List[0].IsActive)
	assert.Equal(t, jmap.ID("sieve-2"), resp.List[1].ID)
	assert.False(t, resp.List[1].IsActive)
}
