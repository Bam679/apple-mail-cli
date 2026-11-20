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
