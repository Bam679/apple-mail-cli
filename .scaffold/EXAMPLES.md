```sh
apple-mail list --unread --since -2d --limit 20          # id  date  from  subject  mailbox
apple-mail search "invoice" --mailbox "iCloud/INBOX" --json --fields id,date,from,subject
apple-mail get 123456 --max-chars 6000                    # plain-text body, capped
apple-mail get 123456 --no-body --json                    # headers + attachment list only
apple-mail mark 123456 123457 --read --dry-run
apple-mail move 123456 --to "iCloud/Archive" --confirm
```
