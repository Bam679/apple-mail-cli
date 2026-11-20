# apple-mail

Read, search, triage and draft Apple Mail from the command line, built for AI coding agents. It cannot send mail.

[![ci](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/Bam679/apple-mail-cli)](https://github.com/Bam679/apple-mail-cli/releases)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-mail-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-mail-cli)
[![licence](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

`apple-mail` controls Mail.app through fixed JXA scripts, reads message lists in bulk, and decodes bodies to plain text in Go. Everything runs on your Mac, and the CLI itself makes no network connections. Triage writes stay off until a policy file allows them. Saving drafts is on by default and can be switched off in the same file, and no command can send, reply or forward mail.

Measured on the 20 newest messages across all inboxes (2026-11-20, v0.1.0, macOS 26.6.2): the default text output was 3,051 bytes and `--json` 4,685 bytes, and the command took 4.0 s. Bytes are not tokens, and a mailbox with long subjects will differ; `apple-mail list --limit 20 | wc -c` shows your own number.

## Why a CLI instead of an MCP server

An MCP server loads its tool definitions into the agent's context in every session, and each result travels back through the model before the agent can act on it. A CLI costs nothing until it is called. The agent learns it from a short skill, filters the output with `jq` or `grep`, and chains several steps in one shell script, for example turning this week's events into reminders in a single run.

`apple-mail` is one of five CLIs with the same output format, exit codes and permission model: [apple-mail](https://github.com/Bam679/apple-mail-cli), [apple-notes](https://github.com/Bam679/apple-notes-cli), [apple-calendar](https://github.com/Bam679/apple-calendar-cli), [apple-reminders](https://github.com/Bam679/apple-reminders-cli) and [paperless](https://github.com/Bam679/paperless-cli). Together they let an agent manage everyday things on a Mac with one set of rules. If your agent cannot run shell commands, an MCP server is the better fit.

## Install

1. Install the CLI:

   ```sh
   brew install Bam679/tap/apple-mail
   ```

   Signed release archives and `go install` are described in [How to install and verify a release](docs/how-to/install-and-verify.md).

2. Check that it can reach Mail:

   ```sh
   apple-mail doctor
   ```

   On the first run macOS asks whether your terminal app may control Mail. See [Security and permissions](#security-and-permissions).

3. Connect Claude Code. The plugin adds a skill that teaches the agent the CLI, and a hook that stops the agent from editing the policy file:

   ```sh
   claude plugin marketplace add Bam679/apple-mail-cli
   claude plugin install apple-mail@apple-mail-cli
   ```

   To get all five tools and the shared `agent-cli` skill from one place, use the [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins) marketplace instead. Install each plugin from one marketplace only.

To remove everything: `claude plugin uninstall apple-mail`, `brew uninstall apple-mail`, and delete `~/.config/agent-cli/apple-mail.json` if you created one.

## Quick start

```sh
apple-mail doctor
```

```text
mail_automation	true	17 mailboxes visible
```

```sh
apple-mail mailboxes                                  # one line per mailbox: path, unread count
apple-mail list --unread --since -2d --limit 20       # id, date, sender, subject, mailbox
apple-mail get <id> --max-chars 4000                  # headers, attachment names, plain-text body
```

Without a policy file every write is refused, a dry run included:

```sh
apple-mail mark 1 --flag --dry-run
```

```text
{"error":{"code":"policy_denied","message":"operation \"mark\" is not allowed by policy","hint":"the policy file must allow this; agents must not edit it","exit":4}}
```

The [tutorial](docs/tutorial/getting-started.md) continues from here to a first write in a container made for the exercise.

## Use it with your agent

With the plugin installed, Claude Code picks up the skill on its own. For other agents (Codex, Cursor, Gemini CLI), add this to `AGENTS.md`:

```markdown
Use `apple-mail` for reading, triaging and drafting the user's email (it cannot send mail). Run `apple-mail schema` once to learn the commands.
Writes need the user's policy file: preview with `--dry-run`, then run with `--confirm`
only after the user agreed. On exit code 4, stop and tell the user.
```

What an agent can rely on: the default output has one item per line with tab-separated fields; `--json` prints `{"items":[…],"count":N,"next_cursor":"…"}` and `--ndjson` one object per line. `--fields`, `--limit` (default 50, at most 1000) and `--max-chars` keep output small, and `--cursor` continues a list. Errors are one JSON line on stderr, and exit codes are fixed:

| Exit | Meaning |
|---|---|
| 0 | success |
| 1 | unexpected error |
| 2 | invalid arguments or flags |
| 3 | item or container not found |
| 4 | denied by the policy; no flag overrides this |
| 5 | write without `--confirm`; the plan is printed, nothing changed |
| 6 | missing macOS permission or credentials |
| 7 | backend did not answer in time |
| 8 | ambiguous name, or the item already exists |

The [CLI reference](docs/reference/cli.md) has every command, flag, field and the JSON shapes.

## How it works

`apple-mail` runs one fixed JXA script through `/usr/bin/osascript`, which controls Mail with Apple Events. Lists read message properties in bulk and resolve one page at a time. `get` fetches the raw message and decodes MIME, character sets and HTML in Go. Your input reaches the script as a single JSON argument, so it never becomes code. Every write command first builds a plan. The binary checks the plan against the policy file, then prints it (`--dry-run`, or no `--confirm`) or applies it (`--confirm`). [Security model](docs/explanation/security-model.md) explains why the checks live in the binary.

## When not to use it

- You want the agent to send or answer mail on its own. The CLI can only save drafts; you send them from Mail.
- You need to search message bodies across large mailboxes. `search` matches subjects and senders only, because reading every body through Mail is slow.
- You need to delete mailboxes. Mail's scripting interface can't; do it in Mail.
- Your agent cannot run shell commands. An MCP server fits better there.

## Security and permissions

`apple-mail` reads mail and saves drafts out of the box. Everything else, and switching drafts off, is set in `~/.config/agent-cli/apple-mail.json`:

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

Operations are `read`, `write` (all writes) or single operations: `draft` (on by default), `mark`, `move`, `delete`, `mailbox-create`. Scopes are glob patterns over mailbox paths `Account/Mailbox/Sub` (local: `On My Mac/Name`) and, for drafts, `draft:<sender address>` or `draft:default`. Deny wins over allow, and an empty `scopes.read.allow` means everything may be read. `apple-mail policy` shows what is allowed now.

The binary reads the policy from that fixed path only. It takes the home directory from the user database, not from `$HOME`, and no flag or environment variable points it elsewhere. A file that is invalid, writable by other users or owned by someone else makes every command fail with exit code 4.

`apple-mail policy --claude-settings` prints Claude Code settings that allow read commands, ask before every write, deny access to the policy directory, and turn on the Bash sandbox with only `apple-mail` excluded. In the sandbox the agent cannot reach Mail through `osascript`, the network or the policy file directly, so `apple-mail` and its policy are the only way to the data. [How to configure permissions](docs/how-to/configure-permissions.md) walks through it.

### macOS privacy permission

`apple-mail` controls Mail.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Mail. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Mail. Full Disk Access is not needed, because the CLI never reads Mail's database files. Mail starts automatically when a command needs it. `apple-mail doctor` shows whether access works; without it, commands exit with code 6.

Two limits come from Mail itself. Its scripting interface cannot delete mailboxes, so there is no `mailbox-delete` command. Reading a body that Mail hasn't downloaded yet can take a while; `--timeout` (default 60 s) ends the wait, and `--no-body` skips the body.

### What leaves your machine

`apple-mail` makes no network connections of its own. It asks Mail for data, and Mail syncs with your accounts as usual. There is no telemetry and no update check. Whatever an agent reads through `apple-mail` is sent to the model provider by the agent, like any other tool output.

### Threat model

| Threat | Mitigation |
|---|---|
| A prompt-injected agent tries destructive writes | Writes are off by default. The binary checks an operation and scope allowlist, needs `--confirm` and limits batch sizes. |
| The agent edits its own permissions | The policy path is fixed. The plugin hook and the recommended deny rules block edits, and the sandbox blocks writes outside the project. |
| Input turns into code | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and fuzz tests check the script and argv shape are invariant. Message bodies are parsed in Go (MIME, HTML to text), never in Mail. |
| Text in the data steers the agent | Output is data only and control characters are stripped. The skill tells agents to treat returned content as untrusted, and read scopes can hide containers. |
| Secrets or personal data leak | No credentials are involved, and the CLI has no send, reply or forward command; `draft` only saves drafts. Attachment contents are never read. Errors never contain credentials. |
| Supply chain | No third-party Go dependencies. Actions are pinned to commit SHAs. CI runs govulncheck, gosec, CodeQL, gitleaks, dependency review and OpenSSF Scorecard. Releases ship signed checksums, SBOMs and build provenance. |

An agent with unrestricted shell access can go around any CLI, for example by calling `osascript` or `curl` directly. Only the sandbox closes that gap.

### Protecting PII with AI agents

- Hide sensitive containers with `scopes.read.deny`. Hidden containers never appear in the output.
- Ask for less. `--fields`, `--limit` and `--max-chars` keep personal data out of the context, and the skill tells agents to use them.
- Mask personal data before it reaches the model. [PasqualePillitteri/claude-privacy-tool](https://github.com/PasqualePillitteri/claude-privacy-tool) adds an offline masking hook to Claude Code, and [gregmos/PII-Shield](https://github.com/gregmos/PII-Shield) anonymises PII through an MCP server, a skill or a CLI.
- Scan tool output with a `PostToolUse` hook: [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks) and [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog) find secrets, and [data-privacy-stack/presidio](https://github.com/data-privacy-stack/presidio) (formerly microsoft/presidio) finds names, addresses, IBANs and phone numbers.
- Keep real data out of bug reports. Redact output or use synthetic examples.

## Commands

| Command | Kind | Description |
|---|---|---|
| `accounts` | read | Mail accounts (`emails` field on request) |
| `mailboxes [--account A]` | read | mailbox paths `Account/Box/Sub` with unread counts |
| `list [--mailbox M]… [--unread] [--flagged] [--since D] [--before D]` | read | newest first, headers only; default: all inboxes |
| `search <text> [same filters] [--subject-only]` | read | subject/sender contains text |
| `get <id> [--no-body]` | read | headers, recipients, attachment list and plain-text body (MIME and HTML decoded in Go) |
| `doctor` | read | permission check |
| `draft --to A… [--cc A…] [--bcc A…] --subject S --body B [--from A]` / `draft -` | write `draft`, on by default | save a new draft in the sender account's Drafts mailbox; never sends |
| `mark <id>… --read\|--unread\|--flag\|--unflag` | write `mark` | triage |
| `move <id>… --to M` | write `move` | needs write scope on source and target; ids change after a move |
| `delete <id>…` | write `delete` | moves to the account's trash |
| `mailbox-create <name>` | write `mailbox-create` | local mailbox under `On My Mac` |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

`draft` is the only write that is on by default; `"operations": {"deny": ["draft"]}` switches it off. There is no send, reply or forward command. Message IDs are Mail's numeric ids.

## Documentation

| Page | Use it when |
|---|---|
| [Getting started](docs/tutorial/getting-started.md) | you are new and want a first read and a first write |
| [How to configure permissions](docs/how-to/configure-permissions.md) | you want an agent to write, or to hide private data |
| [How to install and verify a release](docs/how-to/install-and-verify.md) | you install without Homebrew or want to check signatures |
| [CLI reference](docs/reference/cli.md) | you need a flag, a field, an exit code or the policy format |
| [Security model](docs/explanation/security-model.md) | you want to know why the checks work this way and where they end |
| [Architecture decisions](docs/adr) | you want the reasoning behind the design |

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) has the rules for changes, and [SECURITY.md](SECURITY.md) explains how to report a vulnerability privately.

```sh
git clone https://github.com/Bam679/apple-mail-cli && cd apple-mail-cli
lefthook install   # gofmt, gitleaks and commit-message checks
make check         # everything CI runs
make integration   # tests against the real backend on this machine
```

## Acknowledgements

The approach to fast mailbox listing (bulk property reads, the global message id lookup, parsing bodies outside Mail) follows ideas from [intelligrit/mail-app-cli](https://github.com/intelligrit/mail-app-cli) (MIT). No code was copied.

## Licence

MIT. See [LICENSE](LICENSE).
