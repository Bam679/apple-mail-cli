## macOS privacy (TCC)

`apple-mail` controls Mail.app via Apple Events (`/usr/bin/osascript`). On first use, macOS asks whether **your terminal app** may control **Mail**. If you denied it, enable it under **System Settings > Privacy & Security > Automation > [your terminal] > Mail**. No Full Disk Access is needed, because the Mail database files are never read. Mail must be able to start, and it is launched if it is not running.

Known macOS limitation: Mail's scripting interface can't delete mailboxes (error -10000), so there is no `mailbox-delete` command. Fetching a body whose content is not downloaded yet can be slow. `--timeout` (default 60s) aborts it, and `--no-body` avoids it.
