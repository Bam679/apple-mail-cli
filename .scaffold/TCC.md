## macOS privacy permission

`apple-mail` controls Mail.app with Apple Events through `/usr/bin/osascript`. macOS asks on first use whether the app that runs the CLI (Terminal, iTerm, VS Code or Claude) may control Mail. If you declined, open System Settings > Privacy & Security > Automation, find your terminal app and switch on Mail. Full Disk Access is not needed, because the CLI never reads Mail's database files. Mail starts automatically when a command needs it. `apple-mail doctor` shows whether access works; without it, commands exit with code 6.

Two limits come from Mail itself. Its scripting interface cannot delete mailboxes, so there is no `mailbox-delete` command. Reading a body that Mail hasn't downloaded yet can take a while; `--timeout` (default 60 s) ends the wait, and `--no-body` skips the body.
