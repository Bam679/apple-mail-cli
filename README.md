# apple-mail

Read, search and triage Apple Mail from the command line, built for AI coding agents. It cannot send mail.

[![ci](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-mail-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-mail-cli)
[![release](https://img.shields.io/github/v/release/Bam679/apple-mail-cli)](https://github.com/Bam679/apple-mail-cli/releases)
[![licence](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

## About

`apple-mail` lets an AI agent find, read and sort your email without being able to send any. It controls Mail.app through static JXA scripts, decodes message bodies to plain text, and stays read-only until you allow triage actions in a policy file. The checks run inside the binary, so they apply to every agent, whether that is Claude Code, Codex, Cursor or a script.

`apple-mail` is one of five CLIs that share the same output format, exit codes and permission model: [apple-mail](https://github.com/Bam679/apple-mail-cli), [apple-notes](https://github.com/Bam679/apple-notes-cli), [apple-calendar](https://github.com/Bam679/apple-calendar-cli), [apple-reminders](https://github.com/Bam679/apple-reminders-cli) and [paperless](https://github.com/Bam679/paperless-cli). All of them are available from one Claude Code marketplace, [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins).

## Why a CLI instead of an MCP server

There are MCP servers for some Apple apps, but each tool call goes back through the model, and the tool definitions fill the context before the agent has done anything. With a CLI, the agent learns the commands from a short skill, filters the output with `jq` or `grep`, and chains several steps in one script, for example turning this week's events into reminders in a single run. That is faster and uses far fewer tokens. Single-purpose CLIs exist for some of these apps, but not as a set with one output format and one permission model for Mail, Notes, Calendar, Reminders and Paperless-ngx, which is what you need to let Claude manage everyday things on a Mac.

## Features

- Mailboxes with paths and unread counts, newest-first lists across all inboxes, and search by subject or sender
- Message bodies decoded from MIME and HTML into plain text, with attachment names but never attachment content
- Triage: mark read or unread, flag, move and move to trash
- No send, reply, forward or draft command exists, so an agent cannot send mail through this tool
- Writes stay off until a policy file allows them, per operation and per mailbox
- A Claude Code plugin with a skill for agents and a hook that protects the policy file

## Installation

Requirements: macOS 14+, Apple Silicon and Intel.

With Homebrew:

```sh
brew install Bam679/tap/apple-mail
```

From a release archive. Every release has signed checksums and SLSA build provenance, so check both before you install:

```sh
V=0.1.0 OS=darwin ARCH=arm64
base=https://github.com/Bam679/apple-mail-cli/releases/download/v$V
curl -fsSLO "$base/apple-mail_${V}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$base/checksums.txt"
curl -fsSLO "$base/checksums.txt.sigstore.json"
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Bam679/apple-mail-cli/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
shasum -a 256 --ignore-missing -c checksums.txt
gh attestation verify "apple-mail_${V}_${OS}_${ARCH}.tar.gz" --repo Bam679/apple-mail-cli
tar xzf "apple-mail_${V}_${OS}_${ARCH}.tar.gz" apple-mail && install -m 755 apple-mail ~/.local/bin/
```

From source: `go install github.com/Bam679/apple-mail-cli/cmd/apple-mail@latest`

### Claude Code plugin

The plugin adds a skill that teaches agents the CLI, plus a hook that stops agents from editing the policy file. Install it from one marketplace only:

```sh
# this CLI on its own
claude plugin marketplace add Bam679/apple-mail-cli
claude plugin install apple-mail@apple-mail-cli

# or the whole family, including the shared agent-cli skill
claude plugin marketplace add Bam679/agent-cli-plugins
claude plugin install apple-mail@agent-cli-plugins
```

## Usage

```sh
apple-mail list --unread --since -2d --limit 20          # id  date  from  subject  mailbox
apple-mail search "invoice" --mailbox "iCloud/INBOX" --json --fields id,date,from,subject
apple-mail get 123456 --max-chars 6000                    # plain-text body, capped
apple-mail get 123456 --no-body --json                    # headers + attachment list only
apple-mail mark 123456 123457 --read --dry-run
apple-mail move 123456 --to "iCloud/Archive" --confirm
```

New to the tool? The [getting started tutorial](docs/tutorial/getting-started.md) walks through a first read and a first write.

## Commands

| Command | Kind | Description |
|---|---|---|
| `accounts` | read | Mail accounts (`emails` field on request) |
| `mailboxes [--account A]` | read | mailbox paths `Account/Box/Sub` with unread counts |
| `list [--mailbox M]… [--unread] [--flagged] [--since D] [--before D]` | read | newest first, headers only; default: all inboxes |
| `search <text> [same filters] [--subject-only]` | read | subject/sender contains text |
| `get <id> [--no-body]` | read | headers, recipients, attachment list and plain-text body (MIME and HTML decoded in Go) |
| `doctor` | read | permission check |
| `mark <id>… --read\|--unread\|--flag\|--unflag` | write `mark` | triage |
| `move <id>… --to M` | write `move` | needs write scope on source and target; ids change after a move |
| `delete <id>…` | write `delete` | moves to the account's trash |
| `mailbox-create <name>` | write `mailbox-create` | local mailbox under `On My Mac` |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

There is no send, reply, forward or draft command. Message IDs are Mail's numeric ids.

`apple-mail <command> --help` shows the flags of one command. `apple-mail schema` prints the whole contract as JSON: commands, flags, fields, exit codes and the policy format.

## Output contract

The default output has one item per line with tab-separated fields, no header and no colour. `--json` prints one document (`{"items":[…],"count":N,"next_cursor":"…"}` for lists), and `--ndjson` prints one object per line. Errors are a single JSON line on stderr, for example `{"error":{"code":"policy_denied","message":"…","hint":"…","exit":4}}`.

Agents keep their context small with `--fields`, `--limit` (default 50, at most 1000) and `--max-chars`. When a list continues, the output ends with `# next_cursor=…` (or a `next_cursor` key), and the same command with `--cursor` returns the next page.

| Exit | Name | Meaning |
|---|---|---|
| 0 | ok | success |
| 1 | internal | unexpected error |
| 2 | usage | invalid arguments or flags |
| 3 | not_found | the item or container does not exist |
| 4 | policy_denied | the policy file does not allow this; no flag overrides it |
| 5 | confirm_required | a write without `--confirm`; the plan is printed on stdout |
| 6 | permission_denied | missing macOS privacy permission or credentials |
| 7 | unavailable | the backend did not answer in time |
| 8 | conflict | an ambiguous name, or the item already exists |

The full description is in the [CLI reference](docs/reference/cli.md).

## Permissions

`apple-mail` only reads until you allow writes. A write goes through three checks inside the binary:

1. The policy file `~/.config/agent-cli/apple-mail.json` must allow the operation and every container it touches.
2. `--dry-run` shows the planned change without making it.
3. `--confirm` applies the change. Without it the command prints the plan and exits with code 5.

A policy that allows a few writes looks like this:

```json
   {
     "version": 1,
     "operations": {
       "allow": [
         "read",
         "mark",
         "move"
       ],
       "deny": [
         "delete"
       ]
     },
     "scopes": {
       "read": {
         "allow": [
           "*"
         ],
         "deny": [
           "*/Private*"
         ]
       },
       "write": {
         "allow": [
           "iCloud/INBOX",
           "iCloud/Archive",
           "On My Mac/Agent*"
         ],
         "deny": []
       }
     },
     "limits": {
       "max_write_batch": 20
     }
   }
```

Scopes are glob patterns over mailbox paths `Account/Mailbox/Sub` (local: `On My Mac/Name`). Deny rules win over allow rules, and an empty `scopes.read.allow` means everything may be read. The binary reads the file from a fixed path (the home directory comes from the user database, not `$HOME`), refuses files that other users can write, and treats a broken file as "deny everything". `apple-mail policy` shows what is currently allowed.

[How to configure permissions](docs/how-to/configure-permissions.md) covers the policy file, Claude Code permission rules and the recommended sandbox setup.

## macOS privacy permission

`apple-mail` controls Mail.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Mail. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Mail. Full Disk Access is not needed, because the CLI never reads Mail's database files. Mail starts automatically when a command needs it. `apple-mail doctor` shows whether access works; without it, commands exit with code 6.

Two limits come from Mail itself. Its scripting interface cannot delete mailboxes, so there is no `mailbox-delete` command. Reading a body that Mail hasn't downloaded yet can take a while; `--timeout` (default 60 s) ends the wait, and `--no-body` skips the body.

## Threat model

| Threat | Mitigation |
|---|---|
| A prompt-injected agent tries destructive writes | Writes are off by default. The binary checks an operation and scope allowlist, needs `--confirm` and limits batch sizes. |
| The agent edits its own permissions | The policy path is fixed. The plugin hook and the recommended deny rules block edits, and the Claude Code sandbox blocks writes outside the project. |
| Input turns into code | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and fuzz tests check the script and argv shape are invariant. Message bodies are parsed in Go (MIME, HTML to text), never in Mail. |
| Text in the data steers the agent | Output is data only and control characters are stripped. The skill tells agents to treat returned content as untrusted, and read scopes can hide containers. |
| Leaking secrets or personal data | No credentials are involved, and the CLI has **no send, draft or forward command**. Attachment contents are never read. There is no telemetry and no network traffic. Errors never contain credentials. |
| Supply chain | No third-party Go dependencies. Actions are pinned to commit SHAs. CI runs govulncheck, gosec, CodeQL, gitleaks, dependency review and OpenSSF Scorecard. Releases ship signed checksums, SBOMs and build provenance. |

The CLI cannot stop an agent that has unrestricted shell access from going around it, for example by calling `osascript` directly. The Claude Code sandbox closes that gap. [Security model](docs/explanation/security-model.md) explains the reasoning and the limits.

## Protecting PII with AI agents

Whatever an agent reads through this CLI is sent to the model provider. You can reduce that:

- Hide sensitive containers with `scopes.read.deny`. Hidden containers never appear in the output.
- Ask for less. `--fields`, `--limit` and `--max-chars` keep personal data out of the context, and the skill tells agents to use them.
- Mask personal data before it reaches the model. [PasqualePillitteri/claude-privacy-tool](https://github.com/PasqualePillitteri/claude-privacy-tool) adds an offline masking hook to Claude Code, and [gregmos/PII-Shield](https://github.com/gregmos/PII-Shield) anonymises PII through an MCP server, a skill or a CLI.
- Scan tool output with a Claude Code `PostToolUse` hook before it reaches the model. Tools that work well for this: [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks) for secrets and tokens, [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog) for verified secrets, and [data-privacy-stack/presidio](https://github.com/data-privacy-stack/presidio) (formerly microsoft/presidio) for names, email addresses, IBANs and phone numbers.
- Keep real data out of bug reports. Redact output or use synthetic examples.

## Documentation

- [Getting started](docs/tutorial/getting-started.md) (tutorial)
- [How to configure permissions](docs/how-to/configure-permissions.md)
- [CLI reference](docs/reference/cli.md)
- [Security model](docs/explanation/security-model.md)
- [Architecture decisions](docs/adr)

## Contributing

Contributions are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has the ground rules, and [SECURITY.md](SECURITY.md) explains how to report vulnerabilities privately.

```sh
git clone https://github.com/Bam679/apple-mail-cli && cd apple-mail-cli
lefthook install   # git hooks: gofmt, gitleaks, Conventional Commits
make check         # everything CI runs
make integration   # tests against real data in a sandbox (see CONTRIBUTING.md)
```

## Acknowledgements

The approach to fast mailbox listing (bulk property reads, the global message id lookup, parsing bodies outside Mail) follows ideas from [intelligrit/mail-app-cli](https://github.com/intelligrit/mail-app-cli) (MIT). No code was copied.

## Licence

`apple-mail` is licensed under the MIT Licence. See [LICENSE](LICENSE).
