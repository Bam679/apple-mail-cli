# Getting started with apple-mail

In this tutorial you install `apple-mail`, read some of your Apple Mail data with it, and then let it make one change in a container that exists only for this exercise. It takes about ten minutes. You need a terminal and macOS 14+, Apple Silicon and Intel.

## Install the CLI

```sh
brew install Bam679/tap/apple-mail
apple-mail version
```

## Give it access to Mail

```sh
apple-mail doctor
```

macOS asks whether your terminal may control Mail. Choose OK. The command prints a line with `true` and the number of mailboxes it can see.

## Read your mail

```sh
apple-mail mailboxes
apple-mail list --limit 5
```

The first command lists every mailbox with its path and unread count. The second shows the five newest messages from all inboxes. Pick one of the ids and read it:

```sh
apple-mail get <id> --max-chars 2000
```

You see the headers, the attachment names and the start of the body as plain text.

## Try to change something

```sh
apple-mail mark <id> --flag --dry-run
```

The command exits with code 4: without a policy file the CLI only reads.

## Allow flagging in your inbox

Look up the path of the inbox that holds the message in the output of `apple-mail list` (for example `iCloud/INBOX`) and allow only the `mark` operation there:

```sh
mkdir -p ~/.config/agent-cli
cat > ~/.config/agent-cli/apple-mail.json <<'JSON'
{
  "version": 1,
  "operations": { "allow": ["read", "mark"] },
  "scopes": { "write": { "allow": ["iCloud/INBOX"] } }
}
JSON
chmod 600 ~/.config/agent-cli/apple-mail.json
```

## Flag a message

```sh
apple-mail mark <id> --flag
```

The command exits with code 5 and shows the message it would flag. Run it with `--confirm`, and the flag appears in Mail:

```sh
apple-mail mark <id> --flag --confirm
```

## Clean up

```sh
apple-mail mark <id> --unflag --confirm
rm ~/.config/agent-cli/apple-mail.json
```

## What you have done

You installed `apple-mail` and read data with it. You saw that it refuses to write until a policy allows it, previewed a change with `--dry-run`, and applied it with `--confirm`.

Next steps:

- [How to configure permissions](../how-to/configure-permissions.md) when you want an agent to use the CLI
- [CLI reference](../reference/cli.md) for every command and flag
