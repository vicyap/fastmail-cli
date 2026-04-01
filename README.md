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

```bash
# List mailboxes
fm mailbox list

# List inbox (20 most recent)
fm inbox

# List emails in a mailbox
fm email list --mailbox=Archive --limit=50

# Search emails
fm email search "meeting notes" --from=alice@example.com --after=2026-01-01

# Read an email
fm email read <id>

# Send an email
fm email send --to=bob@example.com --subject="Hello" --body="Hi Bob"

# Manage masked emails
fm masked-email list
fm masked-email create --domain=example.com --description="Newsletter signup"
fm masked-email disable <id>
```

All commands support `--json` for machine-readable output.

## Roadmap

### v0.1 -- Core email and masked email

- [ ] `fm auth login` / `fm auth status`
- [ ] `fm mailbox list`
- [ ] `fm inbox`
- [ ] `fm email list`
- [ ] `fm email search`
- [ ] `fm email read`
- [ ] `fm masked-email list` / `create` / `enable` / `disable` / `delete`

### v0.2 -- Send and organize

- [ ] `fm email send`
- [ ] `fm email move`
- [ ] `fm email delete`
- [ ] Session caching (XDG cache dir)

### v0.3 -- Threads and attachments

- [ ] `fm email thread <id>` -- display full thread
- [ ] Attachment download on `fm email read`
- [ ] Attachment upload on `fm email send`

### v0.4 -- Sieve and filters

- [ ] `fm sieve list` / `get` / `set` -- manage Sieve scripts (RFC 9661)
- [ ] `fm vacation` -- get/set vacation auto-reply

### Future

- Contacts and calendars (pending Fastmail enabling JMAP for these)
- Shell completions (bash, zsh, fish)
- Homebrew tap

## How it works

`fm` talks to Fastmail's [JMAP API](https://www.fastmail.com/dev/) directly. JMAP is a modern, stateless, JSON-over-HTTP protocol created by Fastmail as a replacement for IMAP. The Masked Email commands use Fastmail's [proprietary JMAP extension](https://www.fastmail.com/dev/maskedemail).

Built on [`rockorager/go-jmap`](https://git.sr.ht/~rockorager/go-jmap) for protocol handling.

## License

MIT
