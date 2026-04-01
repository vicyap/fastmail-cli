# Specification

## Overview

`fm` is a command-line interface for Fastmail. It uses the JMAP protocol to interact with Fastmail's API for email, mailbox management, masked email operations, Sieve script management, and vacation auto-reply.

## Authentication

### Token Storage

Tokens are resolved in this order:

1. `--token` flag (per-invocation override)
2. `FASTMAIL_API_TOKEN` environment variable
3. OS keyring (stored by `fm auth login`)

### `fm auth login`

Prompts for an API token and stores it in the OS keyring. The token is validated by fetching the JMAP session endpoint. Falls back to suggesting the env var if keyring is unavailable (headless servers).

Users generate API tokens at: https://app.fastmail.com/settings/security/tokens

Required token scopes:
- `urn:ietf:params:jmap:core`
- `urn:ietf:params:jmap:mail`
- `urn:ietf:params:jmap:submission`
- `urn:ietf:params:jmap:vacationresponse`
- `urn:ietf:params:jmap:sieve`
- `https://www.fastmail.com/dev/maskedemail`

### `fm auth status`

Prints the authenticated user's account info by fetching the JMAP session. Exits non-zero if no valid token is configured.

### `fm auth logout`

Removes the stored API token from the OS keyring.

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

Renders the text body by default. Falls back to HTML body display if `--html` is specified.

Flags:
- `--html` -- render HTML body instead of text
- `--headers` -- show all headers
- `--json` -- JSON output

### `fm email thread <id>`

Displays all emails in a thread. Resolves the thread ID from the given email ID, fetches all email IDs in the thread, then fetches their full content.

JMAP methods: `Email/get` (for threadId) + `Thread/get` + `Email/get` (for full emails)

### `fm email send`

Compose and send an email.

JMAP methods: `Identity/get` + `Mailbox/get` (to find Drafts) + `Email/set` + `EmailSubmission/set` (batched)

The email is created as a draft, then submitted via `EmailSubmission/set`. The `onSuccessUpdateEmail` field removes the `$draft` keyword and moves the email out of Drafts.

Flags:
- `--to <addr>` -- recipient (required, repeatable)
- `--cc <addr>` -- CC recipient (repeatable)
- `--bcc <addr>` -- BCC recipient (repeatable)
- `--subject <text>` -- subject line (required)
- `--body <text>` -- plain text body (reads stdin if omitted)
- `--identity <id>` -- sending identity (default: first identity)
- `--json` -- JSON output (prints created email ID and submission ID)

### `fm email move <id> <mailbox>`

Move an email to a different mailbox.

JMAP methods: `Email/get` (to read current mailboxIds) + `Email/set` (update mailboxIds)

### `fm email delete <id>`

Move an email to Trash. With `--permanent`, destroy the email.

JMAP methods:
- Trash: `Email/get` + `Mailbox/get` (find Trash) + `Email/set` (update mailboxIds)
- Permanent: `Email/set` (destroy)

Flags:
- `--permanent` -- permanently destroy instead of trashing

### `fm email attachment <email-id> [blob-id]`

Download email attachments. If a specific blob-id is given, download only that attachment. Otherwise, download all attachments.

JMAP methods: `Email/get` (for attachment metadata) + blob download endpoint

Flags:
- `-o, --output <dir>` -- output directory (default: current directory)

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

## Sieve Script Commands

RFC 9661: `urn:ietf:params:jmap:sieve`

### `fm sieve list`

Lists all Sieve scripts with name and active status.

JMAP method: `SieveScript/get`

### `fm sieve get <id>`

Displays the content of a Sieve script by downloading its blob.

JMAP methods: `SieveScript/get` (for blobId) + blob download

### `fm sieve set <name> [file]`

Creates a new Sieve script. Reads from stdin if no file is given.

JMAP methods: blob upload + `SieveScript/set` (create)

### `fm sieve activate <id>`

Activates a Sieve script using `onSuccessActivateScript` on `SieveScript/set`.

### `fm sieve deactivate`

Deactivates the current active script using `onSuccessDeactivateScript` on `SieveScript/set`.

### `fm sieve delete <id>`

Destroys a Sieve script. The script must not be active (deactivate first).

JMAP method: `SieveScript/set` (destroy)

### `fm sieve validate [file]`

Validates a Sieve script without storing it. Reads from stdin if no file is given.

JMAP methods: blob upload + `SieveScript/validate`

## Vacation Auto-Reply

### `fm vacation get`

Shows current vacation auto-reply settings.

JMAP method: `VacationResponse/get`

### `fm vacation set`

Enables vacation auto-reply with optional subject, body, and date range.

JMAP method: `VacationResponse/set`

Flags:
- `--subject <text>` -- auto-reply subject
- `--body <text>` -- plain text body
- `--html-body <text>` -- HTML body
- `--from <YYYY-MM-DD>` -- start date
- `--to <YYYY-MM-DD>` -- end date

### `fm vacation disable`

Disables vacation auto-reply.

JMAP method: `VacationResponse/set` (set isEnabled to false)

## Output Formatting

All commands default to human-readable table output. All commands accept `--json` for structured output suitable for scripting and piping.

JSON output writes to stdout. Errors and status messages write to stderr.

## Error Handling

JMAP errors (methodError, requestError) are printed to stderr with the error type and description. The process exits with code 1.

Auth errors (missing token, invalid token, expired session) print a message directing the user to run `fm auth login`.

## Session Caching

The JMAP session response is cached locally (XDG cache dir, `~/.cache/fastmail-cli/session.json`) with a 15-minute TTL. On each request, if the cached session is fresh, skip the session fetch. If stale, re-fetch. The cache can be explicitly invalidated.
