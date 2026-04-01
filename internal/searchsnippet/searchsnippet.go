package searchsnippet

import (
	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	gosnippet "git.sr.ht/~rockorager/go-jmap/mail/searchsnippet"
)

// SearchSnippet re-exports the upstream type.
type SearchSnippet = gosnippet.SearchSnippet

// GetResponse re-exports the upstream type.
type GetResponse = gosnippet.GetResponse

// Get is a corrected version of searchsnippet.Get that returns the correct
// method name. The upstream go-jmap library has a bug where searchsnippet.Get.Name()
// returns "Mailbox/get" instead of "SearchSnippet/get".
type Get struct {
	Account      jmap.ID              `json:"accountId,omitempty"`
	Filter       any          `json:"filter,omitempty"`
	EmailIDs     []jmap.ID            `json:"emailIds,omitzero"`
	ReferenceIDs *jmap.ResultReference `json:"#emailIds,omitempty"`
}

func (m *Get) Name() string         { return "SearchSnippet/get" }
func (m *Get) Requires() []jmap.URI { return []jmap.URI{mail.URI} }
