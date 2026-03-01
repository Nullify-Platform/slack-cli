# slack-cli Agent Usage Guide

You are interacting with Slack workspaces via the `slack-cli` command-line tool. All output is compact JSON with empty fields omitted to minimize token usage.

## Authentication

The tool reads credentials from environment variables. These must be set before any command:

- `SLACK_TOKEN` (required) — Slack API token (`xoxb-*` for bot, `xoxp-*` for user, `xoxc-*` for browser)
- `SLACK_COOKIE_D` (required for `xoxc-*` tokens) — Browser session cookie (`xoxd-*`)
- `SLACK_WORKSPACE_URL` (required for `xoxc-*` tokens) — e.g. `https://myteam.slack.com`

Verify auth works: `slack-cli auth test`

## Discovering Context

### 1. List channels you have access to

```
slack-cli channel list --limit 100
```

Output includes channel `id`, `name`, `is_private`, and `num_members`.

### 2. Read recent messages in a channel

```
slack-cli message list '#channel-name' --limit 25
```

Or by channel ID:

```
slack-cli message list C01234ABC --limit 25
```

Messages are returned in chronological order (oldest first). Each message includes:
- `ts` — unique timestamp identifier
- `thread_ts` — present if the message is part of a thread
- `reply_count` — number of thread replies (only on thread root messages, omitted when 0)
- `author.user_id` — who sent it
- `content` — message text
- `files` — attached file metadata (if any)

### 3. Read a thread

When a message has `reply_count > 0`, fetch the full thread:

```
slack-cli message list '#channel-name' --thread-ts 1772065778.676219
```

### 4. Filter messages by time

Use `--oldest` and `--latest` with Slack timestamps to scope results:

```
slack-cli message list '#channel-name' --oldest 1772200000.000000 --limit 50
```

To compute a timestamp for "N seconds ago", subtract from the current unix epoch.

### 5. Get a single message

By Slack URL:

```
slack-cli message get 'https://myteam.slack.com/archives/C01234/p1772065778676219'
```

By channel + timestamp:

```
slack-cli message get '#channel-name' --ts 1772065778.676219
```

### 6. Resolve user IDs to names

Messages contain user IDs (e.g. `U06JYRAKH9A`), not display names. Resolve them:

```
slack-cli user get U06JYRAKH9A
```

Returns `id`, `name`, `real_name`, `display_name`, `email`, `title`, `tz`.

To list all workspace users:

```
slack-cli user list --limit 200
```

### 7. Search messages

```
slack-cli search messages 'keyword' --limit 20
```

Filter by channel, user, or date range:

```
slack-cli search messages 'deploy' --channel '#engineering' --user '@jules' --after 2026-02-01 --before 2026-02-28
```

### 8. Search files

```
slack-cli search files 'report' --limit 10
```

## Sending and Modifying Messages

### Send a message to a channel

```
slack-cli message send '#channel-name' 'Hello from the agent'
```

### Reply in a thread

```
slack-cli message send '#channel-name' 'Thread reply here' --thread-ts 1772065778.676219
```

Or reply to a Slack URL (automatically threads):

```
slack-cli message send 'https://myteam.slack.com/archives/C01234/p1772065778676219' 'Reply to this message'
```

### Edit a message

```
slack-cli message edit '#channel-name' 'Updated text' --ts 1772065778.676219
```

### Delete a message

```
slack-cli message delete '#channel-name' --ts 1772065778.676219
```

### Add/remove reactions

```
slack-cli message react add '#channel-name' thumbsup --ts 1772065778.676219
slack-cli message react remove '#channel-name' thumbsup --ts 1772065778.676219
```

## Channel Management

### Create a channel

```
slack-cli channel new --name 'new-channel'
slack-cli channel new --name 'private-channel' --private
```

### Invite users to a channel

```
slack-cli channel invite --channel '#channel-name' --users 'U01234,U56789'
```

## Typical Agent Workflow

1. `slack-cli channel list` — discover available channels
2. `slack-cli message list '#channel' --limit 25` — read recent activity
3. For any message with `reply_count > 0` — `slack-cli message list '#channel' --thread-ts <ts>` to read the thread
4. `slack-cli user get <user_id>` — resolve user IDs as needed
5. `slack-cli message send '#channel' 'response'` — reply to the channel
6. `slack-cli message send '#channel' 'response' --thread-ts <ts>` — reply in a specific thread

## Notes

- All output goes to stdout as JSON. Errors go to stderr with exit code 1.
- Channel targets accept `#name`, `name`, or channel IDs (`C01234ABC`).
- User targets accept `@handle`, `handle`, user IDs (`U01234ABC`), or email addresses.
- `--max-body-chars` (default 8000) truncates long message content. Use `-1` for unlimited.
- Timestamps are Slack's unique message identifiers in `seconds.microseconds` format (e.g. `1772065778.676219`).
