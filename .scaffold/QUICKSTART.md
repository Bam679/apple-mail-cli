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
