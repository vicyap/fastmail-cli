package maskedemail

import (
	"encoding/json"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskedEmail_JSONRoundTrip(t *testing.T) {
	me := &MaskedEmail{
		ID:          "me-123",
		Email:       "abc123@fastmail.com",
		State:       "enabled",
		ForDomain:   "example.com",
		Description: "Newsletter signup",
		CreatedAt:   "2026-01-15T10:30:00Z",
		CreatedBy:   "fastmail-cli",
	}

	data, err := json.Marshal(me)
	require.NoError(t, err)

	var decoded MaskedEmail
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, jmap.ID("me-123"), decoded.ID)
	assert.Equal(t, "abc123@fastmail.com", decoded.Email)
	assert.Equal(t, "enabled", decoded.State)
	assert.Equal(t, "example.com", decoded.ForDomain)
	assert.Equal(t, "Newsletter signup", decoded.Description)
}

func TestGet_MethodName(t *testing.T) {
	g := &Get{}
	assert.Equal(t, "MaskedEmail/get", g.Name())
	assert.Equal(t, []jmap.URI{URI}, g.Requires())
}

func TestSet_MethodName(t *testing.T) {
	s := &Set{}
	assert.Equal(t, "MaskedEmail/set", s.Name())
	assert.Equal(t, []jmap.URI{URI}, s.Requires())
}

func TestCapability_URI(t *testing.T) {
	cap := &Capability{}
	assert.Equal(t, URI, cap.URI())
	assert.Equal(t, jmap.URI("https://www.fastmail.com/dev/maskedemail"), cap.URI())

	newCap := cap.New()
	assert.IsType(t, &Capability{}, newCap)
}

func TestSet_CreateSerialization(t *testing.T) {
	s := &Set{
		Account: "u12345",
		Create: map[jmap.ID]*MaskedEmail{
			"new1": {
				ForDomain:   "example.com",
				Description: "Test",
				EmailPrefix: "myprefix",
			},
		},
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Equal(t, "u12345", raw["accountId"])
	create := raw["create"].(map[string]any)
	new1 := create["new1"].(map[string]any)
	assert.Equal(t, "example.com", new1["forDomain"])
	assert.Equal(t, "Test", new1["description"])
	assert.Equal(t, "myprefix", new1["emailPrefix"])
}

func TestSet_UpdateSerialization(t *testing.T) {
	s := &Set{
		Account: "u12345",
		Update: map[jmap.ID]jmap.Patch{
			"me-123": {
				"state": "disabled",
			},
		},
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	update := raw["update"].(map[string]any)
	me123 := update["me-123"].(map[string]any)
	assert.Equal(t, "disabled", me123["state"])
}

func TestGetResponse_Deserialization(t *testing.T) {
	jsonData := `{
		"accountId": "u12345",
		"state": "abc",
		"list": [
			{
				"id": "me-1",
				"email": "test@fastmail.com",
				"state": "enabled",
				"forDomain": "example.com",
				"description": "Test masked email"
			}
		],
		"notFound": []
	}`

	var resp GetResponse
	require.NoError(t, json.Unmarshal([]byte(jsonData), &resp))

	assert.Equal(t, jmap.ID("u12345"), resp.Account)
	assert.Equal(t, "abc", resp.State)
	require.Len(t, resp.List, 1)
	assert.Equal(t, jmap.ID("me-1"), resp.List[0].ID)
	assert.Equal(t, "test@fastmail.com", resp.List[0].Email)
	assert.Equal(t, "enabled", resp.List[0].State)
}

func TestSetResponse_Deserialization(t *testing.T) {
	jsonData := `{
		"accountId": "u12345",
		"oldState": "s1",
		"newState": "s2",
		"created": {
			"new1": {
				"id": "me-456",
				"email": "generated@fastmail.com",
				"state": "pending"
			}
		},
		"notCreated": null
	}`

	var resp SetResponse
	require.NoError(t, json.Unmarshal([]byte(jsonData), &resp))

	assert.Equal(t, "s1", resp.OldState)
	assert.Equal(t, "s2", resp.NewState)
	require.Contains(t, resp.Created, jmap.ID("new1"))
	assert.Equal(t, "generated@fastmail.com", resp.Created["new1"].Email)
}
