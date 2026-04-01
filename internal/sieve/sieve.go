package sieve

import (
	"git.sr.ht/~rockorager/go-jmap"
)

const URI jmap.URI = "urn:ietf:params:jmap:sieve"

func init() {
	jmap.RegisterCapability(&Capability{})
	jmap.RegisterMethod("SieveScript/get", newGetResponse)
	jmap.RegisterMethod("SieveScript/set", newSetResponse)
	jmap.RegisterMethod("SieveScript/query", newQueryResponse)
	jmap.RegisterMethod("SieveScript/validate", newValidateResponse)
}

// Capability describes the Sieve extension capabilities.
type Capability struct {
	MaxSizeScriptName  uint64   `json:"maxSizeScriptName,omitempty"`
	MaxSizeScript      *uint64  `json:"maxSizeScript,omitempty"`
	MaxNumberScripts   *uint64  `json:"maxNumberScripts,omitempty"`
	MaxNumberRedirects *uint64  `json:"maxNumberRedirects,omitempty"`
	SieveExtensions    []string `json:"sieveExtensions,omitzero"`
	Implementation     string   `json:"implementation,omitempty"`
}

func (c *Capability) URI() jmap.URI      { return URI }
func (c *Capability) New() jmap.Capability { return &Capability{} }

// SieveScript represents a Sieve script object per RFC 9661.
type SieveScript struct {
	ID       jmap.ID `json:"id,omitempty"`
	Name     *string `json:"name,omitempty"`
	BlobID   jmap.ID `json:"blobId,omitempty"`
	IsActive bool    `json:"isActive,omitempty"`
}

// Get fetches Sieve scripts.
type Get struct {
	Account    jmap.ID   `json:"accountId,omitempty"`
	IDs        []jmap.ID `json:"ids,omitzero"`
	Properties []string  `json:"properties,omitzero"`
}

func (m *Get) Name() string         { return "SieveScript/get" }
func (m *Get) Requires() []jmap.URI { return []jmap.URI{URI} }

type GetResponse struct {
	Account  jmap.ID        `json:"accountId,omitempty"`
	State    string         `json:"state,omitempty"`
	List     []*SieveScript `json:"list,omitzero"`
	NotFound []jmap.ID      `json:"notFound,omitzero"`
}

func newGetResponse() jmap.MethodResponse { return &GetResponse{} }

// Set creates, updates, or destroys Sieve scripts.
type Set struct {
	Account                    jmap.ID                    `json:"accountId,omitempty"`
	IfInState                  string                     `json:"ifInState,omitempty"`
	Create                     map[jmap.ID]*SieveScript   `json:"create,omitzero"`
	Update                     map[jmap.ID]jmap.Patch     `json:"update,omitzero"`
	Destroy                    []jmap.ID                  `json:"destroy,omitzero"`
	OnSuccessActivateScript    *jmap.ID                   `json:"onSuccessActivateScript,omitempty"`
	OnSuccessDeactivateScript  *bool                      `json:"onSuccessDeactivateScript,omitempty"`
}

func (m *Set) Name() string         { return "SieveScript/set" }
func (m *Set) Requires() []jmap.URI { return []jmap.URI{URI} }

type SetResponse struct {
	Account      jmap.ID                    `json:"accountId,omitempty"`
	OldState     string                     `json:"oldState,omitempty"`
	NewState     string                     `json:"newState,omitempty"`
	Created      map[jmap.ID]*SieveScript   `json:"created,omitzero"`
	Updated      map[jmap.ID]*SieveScript   `json:"updated,omitzero"`
	Destroyed    []jmap.ID                  `json:"destroyed,omitzero"`
	NotCreated   map[jmap.ID]*jmap.SetError `json:"notCreated,omitzero"`
	NotUpdated   map[jmap.ID]*jmap.SetError `json:"notUpdated,omitzero"`
	NotDestroyed map[jmap.ID]*jmap.SetError `json:"notDestroyed,omitzero"`
}

func newSetResponse() jmap.MethodResponse { return &SetResponse{} }

// Query searches for Sieve scripts.
type Query struct {
	Account jmap.ID     `json:"accountId,omitempty"`
	Filter  any `json:"filter,omitempty"`
	Sort    any `json:"sort,omitempty"`
	Limit   uint64      `json:"limit,omitempty"`
}

func (m *Query) Name() string         { return "SieveScript/query" }
func (m *Query) Requires() []jmap.URI { return []jmap.URI{URI} }

type QueryResponse struct {
	Account    jmap.ID   `json:"accountId,omitempty"`
	QueryState string    `json:"queryState,omitempty"`
	IDs        []jmap.ID `json:"ids,omitzero"`
	Total      uint64    `json:"total,omitempty"`
}

func newQueryResponse() jmap.MethodResponse { return &QueryResponse{} }

// Validate checks a Sieve script without storing it.
type Validate struct {
	Account jmap.ID `json:"accountId,omitempty"`
	BlobID  jmap.ID `json:"blobId,omitempty"`
}

func (m *Validate) Name() string         { return "SieveScript/validate" }
func (m *Validate) Requires() []jmap.URI { return []jmap.URI{URI} }

type ValidateResponse struct {
	Account jmap.ID `json:"accountId,omitempty"`
	Error   *jmap.SetError `json:"error,omitempty"`
}

func newValidateResponse() jmap.MethodResponse { return &ValidateResponse{} }
