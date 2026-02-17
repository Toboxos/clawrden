# Slack Bridge for Clawrden

Receives notifications in Slack when commands require HITL approval.

## Setup

### 1. Create Slack Incoming Webhook

1. Go to https://api.slack.com/apps
2. Create a new app or select existing
3. Navigate to "Incoming Webhooks"
4. Activate Incoming Webhooks
5. Click "Add New Webhook to Workspace"
6. Select the channel for notifications
7. Copy the Webhook URL

### 2. Configure Environment

```bash
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
export WARDEN_API_URL="http://localhost:8080"  # Optional, defaults to localhost
```

### 3. Run the Bridge

```bash
# Build first
make build-slack-bridge

# Run
./bin/slack-bridge
```

## How It Works

1. Polls warden API every 5 seconds for pending requests
2. Posts Slack notification for new requests with:
   - Command and arguments
   - Working directory
   - User ID
   - Request ID
   - Instructions to approve/deny
3. Tracks notified requests to avoid duplicates

## Example Notification

```
🔔 New Command Approval Request

`npm install express`
📁 Directory: `/app`
👤 User: `uid:1000`
🆔 Request ID: `abc123`

To approve: `./bin/clawrden-cli approve abc123`
To deny: `./bin/clawrden-cli deny abc123`
Or visit: http://localhost:8080
```

## Docker Deployment

Add to `docker-compose.yml`:

```yaml
slack-bridge:
  image: clawrden-slack-bridge
  environment:
    - SLACK_WEBHOOK_URL=${SLACK_WEBHOOK_URL}
    - WARDEN_API_URL=http://warden:8080
  depends_on:
    - warden
  restart: unless-stopped
```

## Limitations

- Uses webhook (one-way notifications only)
- No interactive buttons (use CLI or web dashboard to approve/deny)
- Polls every 5 seconds (not real-time WebSocket)

## Future Enhancements

- Interactive buttons for approve/deny
- Slash commands support
- Thread-based conversations per request
