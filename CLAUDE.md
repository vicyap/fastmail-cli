# CLAUDE.md

## What This Is

A CLI for Fastmail built in Go, using the JMAP protocol (RFC 8620 / RFC 8621), Fastmail's Masked Email extension, and RFC 9661 for Sieve script management.

## Key Commands

```bash
go build -o fm .          # Build the binary
go test ./...             # Run all tests
go vet ./...              # Static analysis
```

## Architecture

### JMAP Library

Uses `git.sr.ht/~rockorager/go-jmap` for all JMAP protocol handling. Do not reimplement JMAP primitives -- use go-jmap's typed methods, result references, and session management.

**Known upstream bug**: `searchsnippet.Get.Name()` returns `"Mailbox/get"` instead of `"SearchSnippet/get"`. We work around this with our own `internal/searchsnippet` package that re-exports upstream types but provides a corrected `Get` struct.

### Project Layout

```
cmd/                    # cobra commands (one file per command group)
  root.go               # root command, global flags (--json, --token)
  auth.go               # fm auth login/status/logout
  mailbox.go            # fm mailbox list, fm inbox, mailbox lookup helpers
  email.go              # fm email list/search/read, fm inbox
  email_send.go         # fm email send/move/delete
  email_thread.go       # fm email thread
  email_attachment.go   # fm email attachment
  masked_email.go       # fm masked-email CRUD
  sieve.go              # fm sieve list/get/set/activate/deactivate/delete/validate
  vacation.go           # fm vacation get/set/disable
internal/
  client/               # thin wrapper around go-jmap Client (session init, auth, config, cache)
  jmaptest/             # test helpers: mock JMAP server, session fixtures, request parsers
  maskedemail/          # Fastmail Masked Email extension types (not in go-jmap)
  searchsnippet/        # Corrected SearchSnippet/get method (workaround for go-jmap bug)
  sieve/                # RFC 9661 Sieve script JMAP extension types (not in go-jmap)
  output/               # table and JSON output formatters
main.go
```

### Custom JMAP Extensions

The following JMAP extensions are not in go-jmap and are implemented locally using `RegisterCapability` and `RegisterMethod`:

- **Masked Email** (`internal/maskedemail/`): Fastmail-proprietary extension (`https://www.fastmail.com/dev/maskedemail`)
- **Sieve Scripts** (`internal/sieve/`): RFC 9661 (`urn:ietf:params:jmap:sieve`)

### Auth

API token stored in OS keyring via `zalando/go-keyring`. Fallback to `FASTMAIL_API_TOKEN` env var. Resolution order: `--token` flag > env var > keyring.

### Session Caching

JMAP session cached in XDG cache dir (`~/.cache/fastmail-cli/session.json`) with a 15-minute TTL.

### Testing

Tests use `internal/jmaptest` which provides a mock HTTP server that speaks JMAP. Tests validate request construction and response parsing at the HTTP transport level. Mock library: `stretchr/testify` (assert + require).

## Conventions

- CLI framework: `spf13/cobra`
- All commands support `--json` for machine-readable output
- Session endpoint is always `https://api.fastmail.com/jmap/session`
- No contacts or calendar support until Fastmail exposes these via JMAP
- Sending email uses `Email/set` + `EmailSubmission/set` (not SMTP)
- Errors and status messages go to stderr; data output goes to stdout

## References

- JMAP Core: https://www.rfc-editor.org/rfc/rfc8620.html
- JMAP Mail: https://www.rfc-editor.org/rfc/rfc8621.html
- JMAP Sieve: https://www.rfc-editor.org/rfc/rfc9661.html
- Fastmail Masked Email: https://www.fastmail.com/dev/maskedemail
- go-jmap: https://git.sr.ht/~rockorager/go-jmap
- Fastmail API docs: https://www.fastmail.com/dev/
