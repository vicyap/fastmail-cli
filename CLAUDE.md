# CLAUDE.md

## What This Is

A CLI for Fastmail built in Go, using the JMAP protocol (RFC 8620 / RFC 8621) and Fastmail's Masked Email extension.

## Key Commands

```bash
go build -o fm .          # Build the binary
go test ./...             # Run all tests
go vet ./...              # Static analysis
```

## Architecture

### JMAP Library

Uses `git.sr.ht/~rockorager/go-jmap` for all JMAP protocol handling. Do not reimplement JMAP primitives -- use go-jmap's typed methods, result references, and session management.

### Project Layout

```
cmd/           # cobra commands (one file per command group)
internal/
  client/      # thin wrapper around go-jmap Client (session init, auth, config)
  maskedemail/ # Fastmail Masked Email extension types (not in go-jmap)
  output/      # table and JSON output formatters
main.go
```

### Fastmail Masked Email Extension

The Masked Email API (`https://www.fastmail.com/dev/maskedemail`) is a Fastmail-proprietary JMAP extension. It is NOT part of go-jmap. We implement it in `internal/maskedemail/` using go-jmap's `RegisterCapability` and `RegisterMethod` extension points.

### Auth

API token stored in OS keyring via `zalando/go-keyring`. Fallback to `FASTMAIL_API_TOKEN` env var. Resolution order: `--token` flag > env var > keyring.

## Conventions

- CLI framework: `spf13/cobra`
- All commands support `--json` for machine-readable output
- Session endpoint is always `https://api.fastmail.com/jmap/session`
- No contacts or calendar support until Fastmail exposes these via JMAP
- Sending email uses `Email/set` + `EmailSubmission/set` (not SMTP)

## References

- JMAP Core: https://www.rfc-editor.org/rfc/rfc8620.html
- JMAP Mail: https://www.rfc-editor.org/rfc/rfc8621.html
- Fastmail Masked Email: https://www.fastmail.com/dev/maskedemail
- go-jmap: https://git.sr.ht/~rockorager/go-jmap
- Fastmail API docs: https://www.fastmail.com/dev/
