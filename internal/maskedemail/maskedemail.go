package maskedemail

import (
	"git.sr.ht/~rockorager/go-jmap"
)

const URI jmap.URI = "https://www.fastmail.com/dev/maskedemail"

func init() {
	jmap.RegisterCapability(&Capability{})
	jmap.RegisterMethod("MaskedEmail/get", newGetResponse)
	jmap.RegisterMethod("MaskedEmail/set", newSetResponse)
}

type Capability struct{}

func (c *Capability) URI() jmap.URI      { return URI }
func (c *Capability) New() jmap.Capability { return &Capability{} }

// MaskedEmail represents a Fastmail masked email address.
type MaskedEmail struct {
	ID            jmap.ID `json:"id,omitempty"`
	Email         string  `json:"email,omitempty"`
	State         string  `json:"state,omitempty"`
	ForDomain     string  `json:"forDomain,omitempty"`
	Description   string  `json:"description,omitempty"`
	LastMessageAt string  `json:"lastMessageAt,omitempty"`
	CreatedAt     string  `json:"createdAt,omitempty"`
	CreatedBy     string  `json:"createdBy,omitempty"`
	URL           string  `json:"url,omitempty"`
	EmailPrefix   string  `json:"emailPrefix,omitempty"`
}

// Get fetches masked email addresses.
type Get struct {
	Account jmap.ID   `json:"accountId,omitempty"`
	IDs     []jmap.ID `json:"ids,omitzero"`
}

func (m *Get) Name() string         { return "MaskedEmail/get" }
func (m *Get) Requires() []jmap.URI { return []jmap.URI{URI} }

// GetResponse is the response to MaskedEmail/get.
type GetResponse struct {
	Account  jmap.ID        `json:"accountId,omitempty"`
	State    string         `json:"state,omitempty"`
	List     []*MaskedEmail `json:"list,omitzero"`
	NotFound []jmap.ID      `json:"notFound,omitzero"`
}

func newGetResponse() jmap.MethodResponse { return &GetResponse{} }

// Set creates, updates, or destroys masked email addresses.
type Set struct {
	Account   jmap.ID                  `json:"accountId,omitempty"`
	IfInState string                   `json:"ifInState,omitempty"`
	Create    map[jmap.ID]*MaskedEmail `json:"create,omitzero"`
	Update    map[jmap.ID]jmap.Patch   `json:"update,omitzero"`
	Destroy   []jmap.ID                `json:"destroy,omitzero"`
}

func (m *Set) Name() string         { return "MaskedEmail/set" }
func (m *Set) Requires() []jmap.URI { return []jmap.URI{URI} }

// SetResponse is the response to MaskedEmail/set.
type SetResponse struct {
	Account      jmap.ID                    `json:"accountId,omitempty"`
	OldState     string                     `json:"oldState,omitempty"`
	NewState     string                     `json:"newState,omitempty"`
	Created      map[jmap.ID]*MaskedEmail   `json:"created,omitzero"`
	Updated      map[jmap.ID]*MaskedEmail   `json:"updated,omitzero"`
	Destroyed    []jmap.ID                  `json:"destroyed,omitzero"`
	NotCreated   map[jmap.ID]*jmap.SetError `json:"notCreated,omitzero"`
	NotUpdated   map[jmap.ID]*jmap.SetError `json:"notUpdated,omitzero"`
	NotDestroyed map[jmap.ID]*jmap.SetError `json:"notDestroyed,omitzero"`
}

func newSetResponse() jmap.MethodResponse { return &SetResponse{} }
