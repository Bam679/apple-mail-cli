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

## Behaviour notes

- Without `--mailbox`, `list` and `search` cover the inbox of every account.
- `search` matches subject and sender, not body text.
- Bodies are taken from the raw message, decoded in Go (quoted-printable, base64, UTF-8, ISO-8859-1, Windows-1252) and converted from HTML when there is no plain-text part. Hidden elements and tracking pixels are dropped, and links keep their target.
- `delete` moves messages to the account's trash. `mailbox-create` creates local mailboxes under `On My Mac` only.
