# Fastmail JMAP Reference

## Protocol

fm communicates with Fastmail via the JMAP protocol (RFC 8620 / RFC 8621). All requests are JSON-over-HTTP POST to `https://api.fastmail.com/jmap/api/`. Session discovery is at `https://api.fastmail.com/jmap/session`.

## Authentication

Bearer token in the `Authorization` header. Tokens are generated at https://app.fastmail.com/settings/security/tokens.

Required scopes:
- `urn:ietf:params:jmap:core`
- `urn:ietf:params:jmap:mail`
- `urn:ietf:params:jmap:submission`
- `https://www.fastmail.com/dev/maskedemail`

Optional scopes (may not be available on all plans):
- `urn:ietf:params:jmap:vacationresponse`
- `urn:ietf:params:jmap:sieve`

## Key JMAP Methods Used

| Command | JMAP Methods |
|---|---|
| `email list` | `Email/query` + `Email/get` (result references) |
| `email search` | `Email/query` + `Email/get` + `SearchSnippet/get` |
| `email read` | `Email/get` with `fetchTextBodyValues`/`fetchHTMLBodyValues` |
| `email send` | `Identity/get` + `Mailbox/get` + `Email/set` + `EmailSubmission/set` |
| `email move` | `Email/get` + `Email/set` (update `mailboxIds`) |
| `email delete` | `Email/set` (update `mailboxIds` or `destroy`) |
| `email flag/unflag` | `Email/set` (update `keywords/$flagged`) |
| `email thread` | `Email/get` + `Thread/get` + `Email/get` |
| `mailbox list` | `Mailbox/get` |
| `mailbox create/rename/delete` | `Mailbox/set` |
| `masked-email list` | `MaskedEmail/get` |
| `masked-email create/enable/disable/delete` | `MaskedEmail/set` |
| `sieve list/get` | `SieveScript/get` + blob download |
| `sieve set` | blob upload + `SieveScript/set` |
| `vacation get/set` | `VacationResponse/get` / `VacationResponse/set` |

## Masked Email Extension

Capability URI: `https://www.fastmail.com/dev/maskedemail`

This is a Fastmail-proprietary JMAP extension, not part of standard JMAP. States: `pending` (auto-deletes in 24h if unused), `enabled`, `disabled` (mail goes to trash), `deleted` (mail bounced).

## Sieve Scripts (RFC 9661)

Capability URI: `urn:ietf:params:jmap:sieve`

Script content is managed as blobs -- upload the script text, then create/update a `SieveScript` referencing the blob ID. Only one script can be active at a time.

## Result References

fm uses JMAP result references to batch multiple method calls in a single HTTP request. For example, `email list` sends `Email/query` and `Email/get` together, with the get call referencing the query's results via `#ids`.

## Session Caching

The JMAP session is cached at `~/.cache/fastmail-cli/session.json` with a 15-minute TTL.
