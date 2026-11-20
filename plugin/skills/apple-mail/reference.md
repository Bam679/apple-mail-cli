# apple-mail reference

## Fields
`id, date, from, subject, mailbox, read, flagged` (lists). `get` adds `message_id, to, cc, date_sent, size, junk, attachments, body` (and `body_note` when the body was skipped).

- `id` is Mail's numeric message id. It changes when a message is moved. `message_id` (the RFC Message-ID header) is stable.
- `attachments` holds `name (mime, size B)` entries. Attachment content is never read.
- Bodies of messages larger than 10 MB are skipped with a `body_note`.

## Commands
| command | policy op | notes |
|---|---|---|
| `accounts` | read | `--fields name,emails` shows the account addresses |
| `mailboxes [--account]` | read | read-denied mailboxes are omitted |
| `list` | read | `--mailbox` (repeatable, default: all inboxes), `--unread`, `--flagged`, `--since`, `--before` |
| `search <text>` | read | same filters, plus `--subject-only` |
| `get <id> [--no-body]` | read | exit 4 if the mailbox is read-denied |
| `mark <id>…` | mark | `--read`/`--unread`, `--flag`/`--unflag` |
| `move <id>… --to M` | move | write scope needed on the source and target |
| `delete <id>…` | delete | moves to the account's trash |
| `draft` | draft | on by default; `--to/--cc/--bcc` (repeatable), `--subject`, `--body`, `--from`, or `-` with JSON objects. Scope `draft:<from address>` or `draft:default`. Never sends. |
| `mailbox-create <name>` | mailbox-create | local `On My Mac/<name>` only. Mail can't delete mailboxes via scripting. |

Dates: `YYYY-MM-DD`, `YYYY-MM-DDTHH:MM`, `today`, `yesterday`, `-7d`.

## Policy (user-owned, read-only for agents)
`~/.config/agent-cli/apple-mail.json`. Scopes match **mailbox paths**. Drafts are on unless the policy denies `draft` (or `write`). Example: allow triage in the inbox and archive only, never trash, and switch drafts off:
```json
{"version":1,"operations":{"allow":["read","mark","move"],"deny":["delete","draft"]},"scopes":{"read":{"allow":["*"],"deny":["*/Private*"]},"write":{"allow":["iCloud/INBOX","iCloud/Archive"]}}}
```
