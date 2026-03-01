# slack-cli

A Go CLI tool for interacting with Slack, designed for AI agent automation. Produces compact, token-efficient JSON output.

Inspired by [stablyai/agent-slack](https://github.com/stablyai/agent-slack).

## Build

Requires Go 1.23+.

```bash
go build -o slack-cli .
```

## Setup

### 1. Create a Slack App

Go to [api.slack.com/apps](https://api.slack.com/apps) and create a new app for your workspace.

### 2. Add Bot Token Scopes

Under **OAuth & Permissions > Scopes > Bot Token Scopes**, add:

| Scope | Purpose |
|-------|---------|
| `channels:read` | List public channels |
| `channels:history` | Read public channel messages |
| `groups:read` | List private channels |
| `groups:history` | Read private channel messages |
| `im:read` | List DMs |
| `im:history` | Read DM messages |
| `mpim:read` | List group DMs |
| `chat:write` | Send/edit/delete messages |
| `reactions:write` | Add/remove reactions |
| `users:read` | List and look up users |
| `users.profile:read` | Read user profiles |
| `search:read` | Search messages and files |

### 3. Install to Workspace

Click **Install to Workspace** and authorize. Copy the **Bot User OAuth Token** (`xoxb-...`).

### 4. Invite the Bot

Invite the bot to channels it needs access to:

```
/invite @your-bot-name
```

### 5. Set Environment Variables

```bash
export SLACK_TOKEN=xoxb-your-bot-token
```

For browser tokens (advanced):

```bash
export SLACK_TOKEN=xoxc-your-browser-token
export SLACK_COOKIE_D=xoxd-your-cookie
export SLACK_WORKSPACE_URL=https://your-team.slack.com
```

### 6. Verify

```bash
./slack-cli auth test
./slack-cli auth whoami
```

## Commands

```
slack-cli auth test                  Verify credentials
slack-cli auth whoami                Show redacted auth config

slack-cli channel list               List channels
slack-cli channel new                Create a channel
slack-cli channel invite             Invite users to a channel

slack-cli message get <target>       Fetch a single message
slack-cli message list <target>      List channel messages or thread
slack-cli message send <target>      Send a message
slack-cli message edit <target>      Edit a message
slack-cli message delete <target>    Delete a message
slack-cli message react add          Add a reaction
slack-cli message react remove       Remove a reaction

slack-cli search messages <query>    Search messages
slack-cli search files <query>       Search files
slack-cli search all <query>         Search both

slack-cli user list                  List workspace users
slack-cli user get <user>            Get user by ID, handle, or email
```

Run any command with `--help` for full flag documentation.

## Agent Integration

See [SLACK_CLI_USAGE.md](SLACK_CLI_USAGE.md) for a guide on using this tool as an AI agent.

## Project Structure

```
├── main.go                    Entry point
├── cmd/                       Cobra command definitions
│   ├── root.go                Root command + getClient()
│   ├── auth.go                auth test, auth whoami
│   ├── message.go             message get/list/send/edit/delete/react
│   ├── channel.go             channel list/new/invite
│   ├── search.go              search messages/files/all
│   └── user.go                user list/get
├── internal/
│   ├── api/client.go          HTTP client (auth, rate limiting, retry)
│   ├── auth/config.go         Env var auth loading
│   ├── types/types.go         Shared data types
│   ├── slack/                 Slack API operations
│   │   ├── messages.go        Message CRUD + history/thread
│   │   ├── channels.go        Channel list/create/invite/resolve
│   │   ├── users.go           User list/get/resolve
│   │   └── search.go          Search with query builder
│   ├── output/compact.go      JSON output helpers
│   └── urlparse/slackurl.go   Slack URL parsing
```

## License

MIT
