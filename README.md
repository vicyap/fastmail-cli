# fm

A command-line interface for [Fastmail](https://www.fastmail.com) built in Go, using the [JMAP protocol](https://jmap.io/).

## Install

```bash
go install github.com/vicyap/fastmail-cli@latest
```

Or build from source:

```bash
git clone https://github.com/vicyap/fastmail-cli.git
cd fastmail-cli
go build -o fm .
```

## Setup

1. Generate an API token at https://app.fastmail.com/settings/security/tokens
2. Run `fm auth login` and paste the token
3. Verify with `fm auth status`

Or set the `FASTMAIL_API_TOKEN` environment variable.

## Usage

### Authentication

```bash
fm auth login          # Store API token in keyring
fm auth status         # Show authenticated user
fm auth logout         # Remove stored token
```

### Mailboxes

```bash
fm mailbox list        # List all mailboxes with unread/total counts
```

### Email

```bash
# List inbox (20 most recent)
fm inbox

# List emails in a mailbox
fm email list --mailbox=Archive --limit=50

# Search emails
fm email search "meeting notes" --from=alice@example.com --after=2026-01-01

# Read an email
fm email read <id>
fm email read <id> --html       # HTML body
fm email read <id> --headers    # Show all headers

# View full thread
fm email thread <id>

# Send an email
fm email send --to=bob@example.com --subject="Hello" --body="Hi Bob"
fm email send --to=bob@example.com --subject="Hello" < body.txt
fm email send --to=alice@example.com --cc=carol@example.com --subject="Update"

# Move and delete
fm email move <id> Archive
fm email delete <id>              # Move to Trash
fm email delete <id> --permanent  # Permanently destroy

# Download attachments
fm email attachment <email-id>                   # Download all
fm email attachment <email-id> <blob-id>         # Download specific
fm email attachment <email-id> -o ./downloads/   # Specify output dir
```

### Masked Email

```bash
fm masked-email list
fm masked-email list --state=enabled
fm masked-email create --domain=example.com --description="Newsletter signup"
fm masked-email create --prefix=myprefix --domain=shop.com
fm masked-email enable <id>
fm masked-email disable <id>
fm masked-email delete <id>
```

### Sieve Scripts

```bash
fm sieve list                        # List all scripts
fm sieve get <id>                    # Print script content
fm sieve set "my-filter" filter.sieve  # Upload from file
echo 'keep;' | fm sieve set "keep-all"  # Upload from stdin
fm sieve activate <id>               # Activate a script
fm sieve deactivate                  # Deactivate current script
fm sieve delete <id>                 # Delete a script
fm sieve validate filter.sieve       # Validate without storing
```

### Vacation Auto-Reply

```bash
fm vacation get                                        # Show current settings
fm vacation set --subject="Out of office" --body="..."  # Enable
fm vacation set --from=2026-04-01 --to=2026-04-15       # With date range
fm vacation disable                                     # Disable
```

All commands support `--json` for machine-readable output.

## Roadmap

### v0.1 -- Core email and masked email

- [x] `fm auth login` / `fm auth status` / `fm auth logout`
- [x] `fm mailbox list`
- [x] `fm inbox`
- [x] `fm email list`
- [x] `fm email search`
- [x] `fm email read`
- [x] `fm masked-email list` / `create` / `enable` / `disable` / `delete`

### v0.2 -- Send and organize

- [x] `fm email send`
- [x] `fm email move`
- [x] `fm email delete`
- [x] Session caching (XDG cache dir)

### v0.3 -- Threads and attachments

- [x] `fm email thread <id>` -- display full thread
- [x] `fm email attachment` -- download attachments

### v0.4 -- Sieve and filters

- [x] `fm sieve list` / `get` / `set` / `activate` / `deactivate` / `delete` / `validate`
- [x] `fm vacation get` / `set` / `disable`

### Future

- [ ] Integration tests against a real Fastmail account
- [ ] Attachment upload on `fm email send`
- [ ] Contacts and calendars (pending Fastmail enabling JMAP for these)
- [ ] Shell completions (bash, zsh, fish)
- [ ] Homebrew tap

## How it works

`fm` talks to Fastmail's [JMAP API](https://www.fastmail.com/dev/) directly. JMAP is a modern, stateless, JSON-over-HTTP protocol created by Fastmail as a replacement for IMAP. The Masked Email commands use Fastmail's [proprietary JMAP extension](https://www.fastmail.com/dev/maskedemail). Sieve script management uses [RFC 9661](https://www.rfc-editor.org/rfc/rfc9661.html).

Built on [`rockorager/go-jmap`](https://git.sr.ht/~rockorager/go-jmap) for protocol handling.

## License

MIT
