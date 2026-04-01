# Specification

## Overview

`fm` is a command-line interface for Fastmail. It uses the JMAP protocol to interact with Fastmail's API for email, mailbox management, and masked email operations.

## Authentication

### Token Storage

Tokens are resolved in this order:

1. `--token` flag (per-invocation override)
2. `FASTMAIL_API_TOKEN` environment variable
3. OS keyring (stored by `fm auth login`)

### `fm auth login`

Prompts for an API token and stores it in the OS keyring. The token is validated by fetching the JMAP session endpoint.

Users generate API tokens at: https://app.fastmail.com/settings/security/tokens

Required token scopes:
- `urn:ietf:params:jmap:core`
- `urn:ietf:params:jmap:mail`
- `urn:ietf:params:jmap:submission`
- `https://www.fastmail.com/dev/maskedemail`

### `fm auth status`

Prints the authenticated user's account info by fetching the JMAP session. Exits non-zero if no valid token is configured.

## Mailbox Commands

### `fm mailbox list`

Lists all mailboxes with name, unread count, total count, and role (inbox, drafts, sent, trash, etc.).

JMAP method: `Mailbox/get`

Flags:
- `--json` -- JSON output

### `fm inbox`

Sugar for `fm email list --mailbox=Inbox`. Lists the 20 most recent emails in the inbox.

## Email Commands

### `fm email list`

Lists emails in a mailbox, newest first.

JMAP methods: `Email/query` + `Email/get` (batched in one request via result references)

Flags:
- `--mailbox <name|id>` -- mailbox to list (default: Inbox)
- `--limit <n>` -- max results (default: 20)
- `--json` -- JSON output

Fetched properties: `id`, `threadId`, `mailboxIds`, `from`, `to`, `subject`, `receivedAt`, `preview`.

### `fm email search <query>`

Full-text search across all mailboxes.

JMAP methods: `Email/query` + `Email/get` + `SearchSnippet/get` (batched)

Flags:
- `--from <addr>` -- filter by sender
- `--to <addr>` -- filter by recipient
- `--subject <text>` -- filter by subject
- `--before <date>` -- received before date
- `--after <date>` -- received after date
- `--has attachment` -- has attachments
- `--mailbox <name|id>` -- restrict to mailbox
- `--limit <n>` -- max results (default: 20)
- `--json` -- JSON output

### `fm email read <id>`

Displays the full content of an email.

JMAP method: `Email/get` with `bodyValues` and `htmlBody`/`textBody` properties.

Renders the text body by default. Falls back to stripping HTML if no text body exists.

Flags:
- `--html` -- render HTML body instead of text
- `--headers` -- show all headers
- `--json` -- JSON output

### `fm email send`

Compose and send an email.

JMAP methods: `Identity/get` + `Email/set` + `EmailSubmission/set` (batched)

Flags:
- `--to <addr>` -- recipient (required, repeatable)
- `--cc <addr>` -- CC recipient (repeatable)
- `--bcc <addr>` -- BCC recipient (repeatable)
- `--subject <text>` -- subject line (required)
- `--body <text>` -- plain text body (reads stdin if omitted)
- `--identity <id>` -- sending identity (default: primary identity)
- `--json` -- JSON output (prints created email ID and submission ID)

Attachments are out of scope for v0.1.

### `fm email move <id> <mailbox>`

Move an email to a different mailbox.

JMAP method: `Email/set` (update mailboxIds)

### `fm email delete <id>`

Move an email to Trash. With `--permanent`, destroy the email.

JMAP method: `Email/set` (update mailboxIds for trash, destroy for permanent)

Flags:
- `--permanent` -- permanently destroy instead of trashing

## Masked Email Commands

Fastmail extension: `https://www.fastmail.com/dev/maskedemail`

### `fm masked-email list`

Lists all masked email addresses with state, domain, description, and last message date.

JMAP method: `MaskedEmail/get`

Flags:
- `--state <enabled|disabled|pending|deleted>` -- filter by state
- `--json` -- JSON output

### `fm masked-email create`

Creates a new masked email address.

JMAP method: `MaskedEmail/set` (create)

Flags:
- `--domain <domain>` -- associate with a domain
- `--description <text>` -- human-readable description
- `--prefix <prefix>` -- email prefix (optional, Fastmail generates if omitted)

Prints the generated email address on success.

### `fm masked-email enable <id>`

Sets a masked email's state to `enabled`.

### `fm masked-email disable <id>`

Sets a masked email's state to `disabled`.

### `fm masked-email delete <id>`

Sets a masked email's state to `deleted`.

## Output Formatting

All commands default to human-readable table output. All commands accept `--json` for structured output suitable for scripting and piping.

JSON output writes to stdout. Errors and status messages write to stderr.

## Error Handling

JMAP errors (methodError, requestError) are printed to stderr with the error type and description. The process exits with code 1.

Auth errors (missing token, invalid token, expired session) print a message directing the user to run `fm auth login`.

## Session Caching

The JMAP session response is cached locally (XDG cache dir) with a TTL. On each request, if the cached session is fresh, skip the session fetch. If stale, re-fetch. If the API returns a `cannotCalculateChanges` or session-mismatch error, invalidate the cache and retry once.
