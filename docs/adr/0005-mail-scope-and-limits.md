# ADR-0005: Mail: no send capability, path-addressed mailboxes, sandbox limits

- Status: accepted
- Date: 2026-11-20

## Context
Mail is the highest-risk app for agents. Sending is irreversible, and mailbox content is a prime prompt-injection channel. Mail's scripting interface has quirks: message ids change on move; `content()` and `source()` can block while bodies download; mailbox deletion fails with -10000 on current macOS; and messages cannot be created in a mailbox by script. Ideas were taken (not code) from intelligrit/mail-app-cli (MIT): bulk property reads, global `byId`, and body parsing outside Mail.

## Decision
- No send, reply, forward or draft operation exists, neither in the CLI nor in the bridge script. A test asserts this.
- Mailboxes are addressed by path `Account/Box/Sub`, and local mailboxes by `On My Mac/Name`. Inboxes are discovered via the unified inbox (no localized names). The account is detected by `emailAddresses()`.
- Lists use bulk property reads plus one-page `byId` resolution. Bodies come from `source()` (10 MB cap) and are parsed in Go (MIME, charsets, HTML to text), and only for `get`.
- `move` re-resolves the new id by Message-ID.
- Integration tests use a persistent local sandbox `On My Mac/agent-cli-test-sandbox`, created once, because it cannot be removed by script. Real messages are only read, and they serve as targets of writes that the policy must block. Successful triage writes (mark, move, delete) are covered by fake-bridge tests only, because no test message can be created without touching a server account.

## Consequences
+ An agent can never send mail through this tool.
- Mailbox deletion is manual. Triage writes are not end-to-end tested against a real Mail store (tracked as an open TODO).
