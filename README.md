# fm

A command-line interface for [Fastmail](https://www.fastmail.com) built in Go, using the [JMAP protocol](https://jmap.io/).

## Install

### Homebrew (macOS/Linux)

```bash
brew install vicyap/tap/fm
```

### Go

```bash
go install github.com/vicyap/fastmail-cli@latest
```

### Binary

Download from [Releases](https://github.com/vicyap/fastmail-cli/releases) for your platform.

### Build from source

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
fm mailbox list                     # List all mailboxes with unread/total counts
fm mailbox create "Projects"        # Create a new mailbox
fm mailbox rename "Projects" "Work" # Rename a mailbox
fm mailbox delete "Work"            # Delete a mailbox
fm mailbox delete "Work" --force    # Delete mailbox and all emails in it
```

### Identities

```bash
fm identity list       # List sending identities
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
fm email send --to=bob@example.com --subject="Report" --attach=report.pdf --attach=data.csv
fm email send --to=bob@example.com --subject="Newsletter" --html --body="<h1>Hi</h1>"

# Reply and forward
fm email reply <id> --body="Thanks!"
fm email reply <id> --all                  # Reply all
fm email forward <id> --to=carol@example.com

# Flag/unflag and read status
fm email flag <id>
fm email unflag <id>
fm email mark-read <id>
fm email mark-unread <id>

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

### Configuration

```bash
fm config init    # Create default config file
fm config show    # Show current configuration
fm config path    # Print config file path
```

### Shell Completions

```bash
# Bash (add to ~/.bashrc)
eval "$(fm completion bash)"

# Zsh (add to ~/.zshrc)
eval "$(fm completion zsh)"

# Fish
fm completion fish | source
```

Completion scripts are also included in release archives under `completions/`.

### Configuration

Create `~/.config/fm/config.toml`:

```toml
default_identity = "ident-2"   # Default sending identity
default_mailbox = "Archive"     # Default mailbox for fm email list
pager = "less -R"               # Pager for email read/thread
color = true                    # Enable/disable color output
```

All settings are optional. Flags and env vars override config values.

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

### v0.5 -- Polish

- [x] `fm --version` with build-time version injection
- [x] Shell completions (bash, zsh, fish) shipped in release archives
- [x] Homebrew tap (`brew install vicyap/tap/fm`)
- [x] Attachment upload on `fm email send --attach`
- [x] Color output for headers and table headings
- [x] Pager support for `fm email read` and `fm email thread`
- [x] `fm email send --html` for HTML body
- [x] `fm mailbox create` / `rename` / `delete`
- [x] `fm email flag` / `unflag`
- [x] `fm identity list`
- [x] Config file (`~/.config/fm/config.toml`)
- [x] Relative dates in email list output

### v0.6 -- Reply, forward, and agent support

- [x] `fm email reply` / `fm email forward`
- [x] `fm email mark-read` / `fm email mark-unread`
- [x] `fm config init` / `fm config show` / `fm config path`
- [x] `fm describe` -- JSON command schema for AI agents
- [x] Man pages generated via `cobra/doc` and shipped in release archives
- [x] Homebrew tap auth via PAT token
- [x] Agent Skill for Claude Code, Cursor, Codex (`.agents/skills/`)
- [x] Integration tests against a real Fastmail account (22 tests)

### Future
- [ ] Contacts and calendars (pending Fastmail enabling JMAP for these)

## Agent Skill

This repo includes an [Agent Skill](https://agentskills.io) that teaches AI coding agents (Claude Code, Cursor, Codex, Copilot, etc.) how to use `fm`.

### Install with npx (all agents)

```bash
npx skills add vicyap/fastmail-cli
```

This installs the skill into every detected agent's skill directory.

### Manual install

Copy `.agents/skills/fastmail-cli/` into your agent's skill directory:

| Agent | Path |
|---|---|
| Claude Code | `.claude/skills/fastmail-cli/` |
| Cursor, Codex, Copilot | `.agents/skills/fastmail-cli/` |

### What it does

The skill gives agents full context on every `fm` command, JSON output patterns, gotchas, and JMAP protocol details -- so you can just say "search my Fastmail for messages from Alice" and the agent knows how to use `fm` correctly.

## How it works

`fm` talks to Fastmail's [JMAP API](https://www.fastmail.com/dev/) directly. JMAP is a modern, stateless, JSON-over-HTTP protocol created by Fastmail as a replacement for IMAP. The Masked Email commands use Fastmail's [proprietary JMAP extension](https://www.fastmail.com/dev/maskedemail). Sieve script management uses [RFC 9661](https://www.rfc-editor.org/rfc/rfc9661.html).

Built on [`rockorager/go-jmap`](https://git.sr.ht/~rockorager/go-jmap) for protocol handling.

## License

MIT
