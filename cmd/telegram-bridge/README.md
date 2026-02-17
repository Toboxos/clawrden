# Telegram Bridge for Clawrden

Receives notifications in Telegram when commands require HITL approval.

## Setup

### 1. Create Telegram Bot

1. Open Telegram and search for @BotFather
2. Send `/newbot` command
3. Follow instructions to create your bot
4. Copy the Bot Token (looks like `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`)

### 2. Get Your Chat ID

1. Start a conversation with your bot
2. Send any message to the bot
3. Visit: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
4. Find your `chat_id` in the JSON response

OR use @userinfobot:
1. Search for @userinfobot in Telegram
2. Send `/start`
3. Copy your User ID (this is your chat_id)

### 3. Configure Environment

```bash
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
export TELEGRAM_CHAT_ID="123456789"
export WARDEN_API_URL="http://localhost:8080"  # Optional, defaults to localhost
```

### 4. Run the Bridge

```bash
# Build first
make build-telegram-bridge

# Run
./bin/telegram-bridge
```

## How It Works

1. Sends startup message when bridge starts
2. Polls warden API every 5 seconds for pending requests
3. Sends Telegram message for new requests with:
   - Command and arguments (in code block)
   - Working directory
   - User ID
   - Request ID
   - CLI commands to approve/deny
   - Link to web dashboard
4. Tracks notified requests to avoid duplicates

## Example Notification

```
🔔 New Command Approval Request

```
npm install express
```
📁 Directory: `/app`
👤 User: `uid:1000`
🆔 ID: `abc123`

To approve:
`./bin/clawrden-cli approve abc123`

To deny:
`./bin/clawrden-cli deny abc123`

Or visit: http://localhost:8080
```

## Docker Deployment

Add to `docker-compose.yml`:

```yaml
telegram-bridge:
  image: clawrden-telegram-bridge
  environment:
    - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
    - TELEGRAM_CHAT_ID=${TELEGRAM_CHAT_ID}
    - WARDEN_API_URL=http://warden:8080
  depends_on:
    - warden
  restart: unless-stopped
```

## Limitations

- Uses simple polling (not webhook or long polling)
- No inline buttons (use CLI or web dashboard to approve/deny)
- Polls every 5 seconds (not real-time)
- Bot doesn't respond to commands (notification-only)

## Future Enhancements

- Inline keyboard buttons for approve/deny
- Bot commands: /status, /queue, /approve, /deny
- Webhook mode instead of polling
- Group chat support
