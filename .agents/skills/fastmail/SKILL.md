---
name: fastmail
description: Manage Fastmail email, mailboxes, masked emails, sieve filters, and vacation auto-reply. Use when the user wants to interact with Fastmail, manage email, create masked email addresses, configure mail filters, or automate email workflows.
license: MIT
allowed-tools: Bash(fm:*) Read
---

# Fastmail

Manage Fastmail via the `fm` CLI, which talks to Fastmail's JMAP API. All commands support `--json` for machine-readable output.

## Prerequisites

Ensure `fm` is installed and authenticated:

```bash
fm --version    # Verify installation
fm auth status  # Verify authentication
```

If not authenticated, run `fm auth login` and provide a Fastmail API token from https://app.fastmail.com/settings/security/tokens

## Command Reference

Use `fm describe` to get the full command tree as JSON. Use `fm <command> --help` for detailed flag documentation.

### Authentication

```bash
fm auth login     # Prompt for API token, store in keyring
fm auth status    # Show authenticated user and account ID
fm auth logout    # Remove stored token
```

### Email Operations

```bash
# List and search
fm inbox                                    # 20 most recent inbox emails
fm email list --mailbox=Archive --limit=50  # List from specific mailbox
fm email search "query" --from=addr --after=2026-01-01

# Read
fm email read <id>              # Text body (piped through pager)
fm email read <id> --html       # HTML body
fm email read <id> --headers    # All headers

# Thread
fm email thread <id>            # Full thread conversation

# Send
fm email send --to=addr --subject="Subject" --body="Body"
fm email send --to=addr --subject="Subject" < body.txt
fm email send --to=addr --subject="Subject" --attach=file.pdf
fm email send --to=addr --subject="Subject" --html --body="<h1>Hi</h1>"

# Reply and forward
fm email reply <id> --body="Thanks!"
fm email reply <id> --all           # Reply to all recipients
fm email forward <id> --to=addr

# Organize
fm email move <id> Archive
fm email delete <id>                # Move to Trash
fm email delete <id> --permanent    # Permanently destroy
fm email flag <id>                  # Star/flag
fm email unflag <id>
fm email mark-read <id>
fm email mark-unread <id>

# Attachments
fm email attachment <email-id>              # Download all
fm email attachment <email-id> <blob-id>    # Download specific
fm email attachment <email-id> -o ./dir/    # Output directory
```

### Mailboxes

```bash
fm mailbox list                         # List with unread/total counts
fm mailbox create "Name"
fm mailbox rename "OldName" "NewName"
fm mailbox delete "Name"
fm mailbox delete "Name" --force        # Delete with all emails
```

### Masked Email

Fastmail's masked email feature creates disposable addresses.

```bash
fm masked-email list
fm masked-email list --state=enabled    # Filter: enabled/disabled/pending/deleted
fm masked-email create --domain=example.com --description="Signup"
fm masked-email create --prefix=myprefix --domain=shop.com
fm masked-email enable <id>
fm masked-email disable <id>
fm masked-email delete <id>
```

### Identities

```bash
fm identity list    # Show sending identities (name, email, ID)
```

### Sieve Filters

```bash
fm sieve list                              # List scripts with active status
fm sieve get <id>                          # Print script content
fm sieve set "name" filter.sieve           # Upload from file
echo 'keep;' | fm sieve set "keep-all"     # Upload from stdin
fm sieve activate <id>
fm sieve deactivate
fm sieve delete <id>
fm sieve validate filter.sieve             # Check syntax without storing
```

### Vacation Auto-Reply

```bash
fm vacation get
fm vacation set --subject="Out of office" --body="Back on Monday"
fm vacation set --from=2026-04-01 --to=2026-04-15
fm vacation disable
```

### Configuration

```bash
fm config init    # Create default ~/.config/fm/config.toml
fm config show    # Display current settings
fm config path    # Print config file location
```

### Agent Introspection

```bash
fm describe       # Full command tree as JSON
```

## Working with JSON Output

All commands accept `--json` for structured output suitable for piping and scripting:

```bash
# Get inbox as JSON
fm inbox --json

# Extract email IDs with jq
fm email list --json | jq -r '.[].id'

# Get masked email addresses
fm masked-email list --json | jq -r '.[].email'

# Get mailbox names and unread counts
fm mailbox list --json | jq '.[] | {name, unreadEmails}'
```

## Gotchas

- `fm email search` requires at least one positional query argument. Filters alone won't work -- pass `""` as an empty query if you only want filter flags.
- `fm email read` pipes through `$PAGER` (or `less`) when stdout is a terminal. Use `--json` or pipe to another command to skip the pager.
- Masked email `delete` sets state to `deleted` (bounces mail) -- it doesn't destroy the record. There's no undo from `deleted`.
- Sieve scripts must be deactivated before deletion. `fm sieve delete` will fail on the active script.
- The `--mailbox` flag accepts both mailbox names and IDs. Names are matched case-sensitively.
- Token resolution order: `--token` flag > `FASTMAIL_API_TOKEN` env var > OS keyring.
- On headless servers without a keyring, use the `FASTMAIL_API_TOKEN` environment variable.

## Fastmail JMAP Details

For advanced usage or debugging, see [references/jmap.md](references/jmap.md).
