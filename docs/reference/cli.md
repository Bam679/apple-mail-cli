# apple-mail CLI reference

`apple-mail` version 0.x. `apple-mail schema` prints the same information as JSON and is always current for the installed version.

## Synopsis

```text
apple-mail <command> [arguments] [flags]
```

Flags can appear before or after arguments. Every flag has two dashes. `--` ends flag parsing.

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

## Global flags

| Flag | Type | Description |
|---|---|---|
| `--json` | bool | Print one JSON document. |
| `--ndjson` | bool | Print one JSON object per line. |
| `--fields a,b` | string | Select and order fields. Unknown names are a usage error. |
| `--limit N` | int | Maximum number of items. Default 50, at most 1000. |
| `--cursor C` | string | Continue a list from a `next_cursor`. A cursor only works with the same command and flags. |
| `--max-chars N` | int | Cap the size of stdout. Lists drop whole items, single items shorten their longest text field. |
| `--dry-run` | bool | Writes only. Print the plan, change nothing. |
| `--confirm` | bool | Writes only. Apply the change. |
| `--timeout D` | duration | Abort after this time (default `60s`). |
| `--help` | bool | Show help. |

## Output formats

| Format | Lists | Single items |
|---|---|---|
| default | One line per item, tab-separated default fields, `-` for empty values. Trailer lines start with `#`: `# next_cursor=…`, `# truncated: …` | `field: value` lines |
| `--json` | `{"items":[…],"count":N,"next_cursor":"…","truncated":true}` | One object; `"truncated":true` when shortened |
| `--ndjson` | One object per line, then `{"_meta":{"next_cursor":…,"truncated":…}}` when relevant | One object |

Write commands print the changed items with an `op` field. With `--dry-run`, or without `--confirm`, they print the planned items with `"dry_run":true`.

Text output never contains control characters, tabs or newlines inside a value, or bidirectional override characters.

Errors are one JSON line on stderr:

```json
{"error":{"code":"policy_denied","message":"writing to \"Private\" is not allowed by policy","hint":"…","exit":4}}
```

## Exit codes

| Code | Name | Meaning |
|---|---|---|
| 0 | ok | Success. |
| 1 | internal | Unexpected error. |
| 2 | usage | Invalid arguments or flags. |
| 3 | not_found | The item or container does not exist. |
| 4 | policy_denied | The policy does not allow the operation or scope, or the policy file is invalid. No flag overrides this. |
| 5 | confirm_required | A write without `--confirm`. The plan is printed on stdout. |
| 6 | permission_denied | Missing macOS privacy permission, or invalid credentials. |
| 7 | unavailable | The backend did not answer, or the timeout expired. |
| 8 | conflict | An ambiguous name, or the item already exists. |

## Stdin

Write commands that take `-` as an argument read JSON objects from stdin: one object, an array of objects, or one object per line. Input is limited to 4 MiB, and unknown fields are rejected.

## Dates

Date flags accept `YYYY-MM-DD` (a whole day), `YYYY-MM-DDTHH:MM` and `YYYY-MM-DD HH:MM` in local time, RFC 3339, `today`, `tomorrow`, `yesterday`, `+Nd` and `-Nd`.

## Fields

| Field | Description |
|---|---|
| `id` | Mail's numeric message id. It changes when a message is moved; the output of `move` carries the new id. |
| `date`, `date_sent` | Received and sent time, RFC 3339. |
| `from`, `to`, `cc` | Sender and recipient addresses. |
| `subject` | Subject line. |
| `mailbox` | Mailbox path, `Account/Mailbox/Sub` or `On My Mac/Name` for local mailboxes. |
| `read`, `flagged`, `junk` | Message state. |
| `message_id` | The RFC `Message-ID` header. Stable across moves. |
| `size` | Size in bytes. |
| `attachments` | `name (MIME type, size B)` for each attachment. |
| `body` | Plain-text body (`get` only). Messages over 10 MB have a `body_note` instead. |

The default text columns are `id date from subject mailbox`. Lists are sorted by received date, newest first.

## Scopes

Policy scopes match mailbox paths. `move` needs write scope on both the source and the target mailbox.

Drafts use the scope `draft:<sender address>` (lower case), or `draft:default` when `--from` is not given and Mail picks the sender. `draft` is on by default: it runs unless `operations.deny` lists `draft` or `write`, or a `scopes.write.deny` pattern matches its scope. `"scopes": {"write": {"deny": ["draft:me@work.example"]}}` blocks drafts from one address only.

## Behaviour notes

- Without `--mailbox`, `list` and `search` cover the inbox of every account.
- `search` matches subject and sender, not body text.
- Bodies are taken from the raw message, decoded in Go (quoted-printable, base64, UTF-8, ISO-8859-1, Windows-1252) and converted from HTML when there is no plain-text part. Hidden elements and tracking pixels are dropped, and links keep their target.
- `draft` saves a new message in the Drafts mailbox of the sender's account, with `visible` off, and never sends it. Recipients are checked as RFC 5322 addresses; `--from` must be an address of one of your Mail accounts.
- `delete` moves messages to the account's trash. `mailbox-create` creates local mailboxes under `On My Mac` only.

## Policy file

Path: `~/.config/agent-cli/apple-mail.json`. The home directory comes from the user database, not from `$HOME`. No flag or environment variable selects a different file.

| Key | Type | Meaning |
|---|---|---|
| `version` | int | Must be `1`. |
| `operations.allow` | string list | `read`, `write` (every write operation), or single operations: `draft` (on by default), `mark`, `move`, `delete`, `mailbox-create`. |
| `operations.deny` | string list | Denied operations. Deny wins over allow. |
| `scopes.read.allow` | glob list | Containers that may be read. Empty means all. |
| `scopes.read.deny` | glob list | Containers hidden from every read. |
| `scopes.write.allow` | glob list | Containers that may be changed. Empty means none. |
| `scopes.write.deny` | glob list | Containers that may never be changed. |
| `limits.max_write_batch` | int | Maximum number of items one write command may change (default 100). |

Scope strings are mailbox paths `Account/Mailbox/Sub` (local: `On My Mac/Name`) and, for drafts, `draft:<sender address>` or `draft:default`. In patterns `*` matches any text, including `/`; matching is case-sensitive.

Without a file, the default policy allows `read` on everything and no writes. The CLI rejects files that are not regular files, that are writable by group or others, that another user owns, or that contain unknown keys or invalid JSON. A rejected file denies every command except `schema`, `policy`, `version`, `help` and `guard`.

## Built-in commands

| Command | Description |
|---|---|
| `schema` | The machine-readable contract as JSON. |
| `policy` | The effective policy and its path. `--example` prints a template, and `--claude-settings` prints recommended Claude Code settings. |
| `guard` | Claude Code `PreToolUse` hook. It reads the hook JSON on stdin and denies tool calls that would modify agent-cli policy files. |
| `version` | Name and version. |
| `help` | Usage. |
