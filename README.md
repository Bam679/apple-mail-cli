# apple-mail

[![ci](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-mail-cli/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-mail-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-mail-cli)
[![release](https://img.shields.io/github/v/release/Bam679/apple-mail-cli)](https://github.com/Bam679/apple-mail-cli/releases)

**Apple Mail for AI coding agents.** A small, dependency-free Go CLI with compact, machine-readable output. It is **read-only by default**, and writes must be allowed by a policy file that the binary enforces. It also ships as a Claude Code plugin with a skill that teaches agents how to use it.

Part of the agent-cli family: [apple-mail](https://github.com/Bam679/apple-mail-cli) · [apple-notes](https://github.com/Bam679/apple-notes-cli) · [apple-calendar](https://github.com/Bam679/apple-calendar-cli) · [apple-reminders](https://github.com/Bam679/apple-reminders-cli) · [paperless](https://github.com/Bam679/paperless-cli). All of them are available from one marketplace: [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins).

## Install

**Binary** (macOS 14+, Apple Silicon and Intel): download from [Releases](https://github.com/Bam679/apple-mail-cli/releases), verify it, and put it on your `PATH`:

```sh
V=0.1.0; OS=darwin; ARCH=arm64
curl -fsSLO https://github.com/Bam679/apple-mail-cli/releases/download/v$V/apple-mail_${V}_${OS}_${ARCH}.tar.gz
curl -fsSLO https://github.com/Bam679/apple-mail-cli/releases/download/v$V/checksums.txt
curl -fsSLO https://github.com/Bam679/apple-mail-cli/releases/download/v$V/checksums.txt.sigstore.json
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Bam679/apple-mail-cli/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
shasum -a 256 --ignore-missing -c checksums.txt
gh attestation verify apple-mail_${V}_${OS}_${ARCH}.tar.gz --repo Bam679/apple-mail-cli   # SLSA provenance
tar xzf apple-mail_${V}_${OS}_${ARCH}.tar.gz apple-mail && install -m 755 apple-mail ~/.local/bin/
```

**From source:** `go install github.com/Bam679/apple-mail-cli/cmd/apple-mail@latest`

**Claude Code plugin** (skill + guard hook). Install it from one source only:

```sh
# just this CLI
claude plugin marketplace add Bam679/apple-mail-cli
claude plugin install apple-mail@apple-mail-cli
# or everything, including the shared agent-cli skill
claude plugin marketplace add Bam679/agent-cli-plugins
claude plugin install apple-mail@agent-cli-plugins
```

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
| `move <id>… --to M` | write `move` | needs write scope on source **and** target; ids change after a move |
| `delete <id>…` | write `delete` | moves to the account's trash |
| `mailbox-create <name>` | write `mailbox-create` | local mailbox under `On My Mac` |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

**There is no send, reply, forward or draft command, by design.** Message IDs are Mail's numeric ids.

Run `apple-mail <command> --help` for flags, or `apple-mail schema` for the full machine-readable contract (commands, flags, fields, exit codes, policy).

## Agent examples

```sh
apple-mail list --unread --since -2d --limit 20          # id  date  from  subject  mailbox
apple-mail search "invoice" --mailbox "iCloud/INBOX" --json --fields id,date,from,subject
apple-mail get 123456 --max-chars 6000                    # plain-text body, capped
apple-mail get 123456 --no-body --json                    # headers + attachment list only
apple-mail mark 123456 123457 --read --dry-run
apple-mail move 123456 --to "iCloud/Archive" --confirm
```

## Output contract

| Mode | Shape |
|---|---|
| default | One item per line, tab-separated default fields, `-` for empty values, no header, no color. Trailer lines start with `#` (`# next_cursor=…`, `# truncated: …`). |
| `--json` | Lists: `{"items":[…],"count":N,"next_cursor":"…","truncated":true}`. Single items: one object. |
| `--ndjson` | One object per line, plus an optional final `{"_meta":{"next_cursor":…,"truncated":…}}`. |
| errors | Exactly one JSON line on **stderr**: `{"error":{"code":"policy_denied","message":"…","hint":"…","exit":4}}` |

- `--fields a,b`: choose and order fields (validated). `--limit N` (default 50, capped by policy). `--cursor C` continues from `next_cursor`. Cursors are opaque and bound to their query.
- `--max-chars N` caps stdout. Lists drop whole items; single items shorten their longest text field and set `"truncated":true`.
- IDs are stable native identifiers and can be passed back to `get`, `update` and `delete`.
- Stdin: write commands that show `<input>` accept `-` plus JSON / NDJSON objects (max 4 MiB, unknown fields rejected).
- Control characters and bidi overrides are stripped from text output, so a value can never break the one-line-per-item contract or inject terminal escapes.

| Exit | Name | Meaning |
|---|---|---|
| 0 | ok | success |
| 1 | internal | unexpected error |
| 2 | usage | invalid arguments or flags |
| 3 | not_found | item or container does not exist |
| 4 | policy_denied | denied by the policy file (flags cannot override this) |
| 5 | confirm_required | write without `--confirm` (preview with `--dry-run`) |
| 6 | permission_denied | macOS privacy permission (TCC) or authentication failure |
| 7 | unavailable | backend not reachable or timed out |
| 8 | conflict | ambiguous name or already exists |

## Permissions and policy

Writes pass through **three gates, in this order**. Each one is enforced in the binary.

1. **Policy file.** With no policy file, `apple-mail` is read-only. To allow writes, create `~/.config/agent-cli/apple-mail.json` (mode `600`):

   ```json
   {
     "version": 1,
     "operations": { "allow": ["read", "mark", "move"], "deny": ["delete"] },
     "scopes": {
       "read":  { "allow": ["*"], "deny": ["*/Private*"] },
       "write": { "allow": ["iCloud/INBOX", "iCloud/Archive", "On My Mac/Agent*"], "deny": [] }
     },
     "limits": { "max_results": 200, "max_write_batch": 20 }
   }
   ```

   - `operations.allow`: `read`, `write` (all writes) or single ops (`mark`, `move`, `delete`, `mailbox-create`). `deny` always wins.
   - `scopes.read` / `scopes.write`: glob patterns (`*` wildcard) over mailbox paths `Account/Mailbox/Sub` (local: `On My Mac/Name`). Read-deny hides containers from lists and blocks `get`.
   - `limits.max_results` caps `--limit`. `limits.max_write_batch` caps the number of items per write.
   - **Not bypassable:** the path is fixed. The home directory comes from the user database, not `$HOME`, and no flag or env var selects another file. Invalid, group- or world-writable, or foreign-owned files make the CLI **fail closed** (exit 4).
   - **Admin lock:** `/Library/Application Support/agent-cli/apple-mail.json` must be owned by root. It is intersected with the user file, so it can only restrict further, and an agent running as you cannot change it.
   - `apple-mail policy` shows the effective layers, and `apple-mail policy --example` prints a template.
2. **`--dry-run`** prints the planned change (policy-checked as well) and changes nothing.
3. **`--confirm`** is required to change data. Without it you get exit 5.

**Claude Code rules (defence in depth).** The plugin's `PreToolUse` hook (`apple-mail guard`) blocks Edit, Write and Bash calls that try to modify agent-cli policy files. To get recommended permission rules (read commands allowed, writes ask, policy files denied), run the command below and merge its output into `~/.claude/settings.json`:

```sh
apple-mail policy --claude-settings
```

## macOS privacy (TCC)

`apple-mail` controls Mail.app via Apple Events (`/usr/bin/osascript`). On first use, macOS asks whether **your terminal app** may control **Mail**. If you denied it, enable it under **System Settings > Privacy & Security > Automation > [your terminal] > Mail**. No Full Disk Access is needed, because the Mail database files are never read. Mail must be able to start, and it is launched if it is not running.

Known macOS limitation: Mail's scripting interface can't delete mailboxes (error -10000), so there is no `mailbox-delete` command. Fetching a body whose content is not downloaded yet can be slow. `--timeout` (default 60s) aborts it, and `--no-body` avoids it.

## Threat model

| Threat | Mitigation |
|---|---|
| Prompt-injected agent tries destructive writes | Read-only default. Binary-enforced policy with op and scope allowlists, `--confirm`, batch limits, no bypass flags. |
| Agent edits its own policy | Fixed path ignoring `$HOME`, ownership and permission checks, guard hook, recommended deny rules, root-owned system policy for hard guarantees. |
| Script or command injection through data | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and fuzz tests check the script and argv shape are invariant. Message bodies are parsed in Go (MIME, HTML to text), never in Mail. |
| Malicious content in data steering the agent | Output is data only. Text is sanitized (no control or escape sequences). Read scopes can hide sensitive containers. Treat all returned content as untrusted. |
| Secret or PII leakage | No credentials are involved, and the CLI has **no send, draft or forward command**. Attachment contents are never read. No telemetry, no network calls. Errors never echo credentials. |
| Supply chain | Zero third-party Go dependencies. SHA-pinned actions, govulncheck, gosec, CodeQL, gitleaks, dependency review, OpenSSF Scorecard. Releases signed with cosign, with SBOMs and SLSA provenance. |

Out of scope: an agent with unrestricted shell access and your full user privileges can still act outside this CLI (for example by calling `osascript` directly). Use the Claude Code sandbox and permission rules to constrain the agent itself.

## Protecting PII with AI agents

Everything an agent reads through this CLI is sent to the model provider. Reduce what leaves your machine:

- **Scope reads.** Deny sensitive containers in `scopes.read.deny` (for example private folders, health or finance tags). Hidden containers never reach the agent.
- **Keep outputs small.** Prefer `--fields` and `--limit`, and use `--max-chars` instead of dumping full bodies.
- **Add hooks that scan before data leaves.** A Claude Code `PostToolUse` hook can pipe tool output through a detector and block or redact it:
  - [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks): secrets (API keys, tokens) in text and repos
  - [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog): verified secret detection
  - [microsoft/presidio](https://github.com/microsoft/presidio): PII detection and anonymization (names, emails, IBANs, phone numbers)
- **Never paste outputs into issues.** Bug reports should contain redacted or synthetic data only.
- **Review the policy regularly.** `apple-mail policy` shows exactly what the agent may touch.

## Development

```sh
make check        # fmt, vet, golangci-lint, race tests + coverage gate, govulncheck, gosec, gitleaks, plugin validate
make integration  # real sandbox tests on this machine
lefthook install  # git hooks (gofmt, gitleaks, Conventional Commits)
```

Decisions are recorded in [docs/adr](docs/adr). See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE)
