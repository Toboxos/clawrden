# Chat Integration Guide

**Status:** Design Complete - Implementation Ready
**Last Updated:** 2026-02-17

## Overview

Clawrden can integrate with chat platforms (Slack, Telegram, Discord) to enable:
1. **HITL Approval** - Approve/deny commands from chat
2. **Notifications** - Get alerted when new requests arrive
3. **Audit Query** - View command history via chat commands
4. **Status Monitoring** - Check warden health from chat

This document provides a **quick implementation** using existing HTTP API with chat webhooks/bots.

---

## Architecture

```
┌─────────────────────┐
│   Warden Server     │
│   HTTP API :8080    │
│                     │
│   /api/queue        │
│   /api/approve      │
│   /api/deny         │
│   /api/history      │
└──────────┬──────────┘
           │
           │ HTTP/JSON
           │
           ▼
┌─────────────────────┐
│   Chat Bridge       │
│   (Go microservice) │
│                     │
│   • Polls warden    │
│   • Sends messages  │
│   • Handles replies │
└──────────┬──────────┘
           │
           │ Webhook/Bot API
           │
           ▼
┌─────────────────────┐
│   Chat Platform     │
│   Slack/Telegram    │
└─────────────────────┘
```

---

## Quick Implementation: Slack Integration

### Prerequisites

1. **Slack App Setup**
   - Create app at https://api.slack.com/apps
   - Enable "Bot Token Scopes": `chat:write`, `commands`
   - Install app to workspace
   - Copy Bot Token (starts with `xoxb-`)

2. **Environment Variables**
   ```bash
   export SLACK_BOT_TOKEN="xoxb-your-token-here"
   export SLACK_CHANNEL="#clawrden-approvals"
   export WARDEN_API_URL="http://localhost:8080"
   ```

### Implementation: Slack Bridge Service

Create `cmd/slack-bridge/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/slack-go/slack"
)

type WardenClient struct {
    baseURL string
    client  *http.Client
}

func NewWardenClient(baseURL string) *WardenClient {
    return &WardenClient{
        baseURL: baseURL,
        client:  &http.Client{Timeout: 10 * time.Second},
    }
}

func (w *WardenClient) GetQueue(ctx context.Context) ([]QueueItem, error) {
    resp, err := w.client.Get(w.baseURL + "/api/queue")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var items []QueueItem
    if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
        return nil, err
    }
    return items, nil
}

func (w *WardenClient) Approve(ctx context.Context, id string) error {
    url := fmt.Sprintf("%s/api/queue/%s/approve", w.baseURL, id)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
    _, err := w.client.Do(req)
    return err
}

func (w *WardenClient) Deny(ctx context.Context, id string) error {
    url := fmt.Sprintf("%s/api/queue/%s/deny", w.baseURL, id)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
    _, err := w.client.Do(req)
    return err
}

type QueueItem struct {
    ID       string   `json:"id"`
    Command  string   `json:"command"`
    Args     []string `json:"args"`
    Cwd      string   `json:"cwd"`
    Identity struct {
        UID int `json:"uid"`
        GID int `json:"gid"`
    } `json:"identity"`
}

func main() {
    token := os.Getenv("SLACK_BOT_TOKEN")
    channel := os.Getenv("SLACK_CHANNEL")
    wardenURL := os.Getenv("WARDEN_API_URL")

    if token == "" || channel == "" || wardenURL == "" {
        log.Fatal("Missing required environment variables")
    }

    api := slack.New(token)
    warden := NewWardenClient(wardenURL)

    // Track which requests we've already notified about
    notified := make(map[string]bool)

    log.Println("Slack bridge started. Polling warden every 5 seconds...")

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        ctx := context.Background()
        items, err := warden.GetQueue(ctx)
        if err != nil {
            log.Printf("Error fetching queue: %v", err)
            continue
        }

        for _, item := range items {
            if notified[item.ID] {
                continue
            }

            // Format command
            cmdStr := fmt.Sprintf("%s %s", item.Command,
                strings.Join(item.Args, " "))

            // Send Slack message with action buttons
            _, _, err := api.PostMessage(
                channel,
                slack.MsgOptionText(
                    fmt.Sprintf("🔔 *New Command Approval Request*\n"+
                        "```%s```\n"+
                        "📁 Working Directory: `%s`\n"+
                        "👤 User: `uid:%d`\n"+
                        "🆔 Request ID: `%s`",
                        cmdStr, item.Cwd, item.Identity.UID, item.ID),
                    false,
                ),
                slack.MsgOptionAttachments(
                    slack.Attachment{
                        CallbackID: item.ID,
                        Actions: []slack.AttachmentAction{
                            {
                                Name:  "approve",
                                Text:  "✅ Approve",
                                Type:  "button",
                                Value: item.ID,
                                Style: "primary",
                            },
                            {
                                Name:  "deny",
                                Text:  "❌ Deny",
                                Type:  "button",
                                Value: item.ID,
                                Style: "danger",
                            },
                        },
                    },
                ),
            )

            if err != nil {
                log.Printf("Error posting to Slack: %v", err)
                continue
            }

            notified[item.ID] = true
        }

        // Clean up notified map for completed requests
        for id := range notified {
            found := false
            for _, item := range items {
                if item.ID == id {
                    found = true
                    break
                }
            }
            if !found {
                delete(notified, id)
            }
        }
    }
}
```

### Add Interactive Endpoint (Handles Button Clicks)

Add to `cmd/slack-bridge/main.go`:

```go
func handleSlackActions(warden *WardenClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Parse Slack payload
        var payload slack.InteractionCallback
        err := json.Unmarshal([]byte(r.FormValue("payload")), &payload)
        if err != nil {
            http.Error(w, "Invalid payload", http.StatusBadRequest)
            return
        }

        if len(payload.ActionCallback.AttachmentActions) == 0 {
            return
        }

        action := payload.ActionCallback.AttachmentActions[0]
        requestID := action.Value

        ctx := context.Background()

        switch action.Name {
        case "approve":
            if err := warden.Approve(ctx, requestID); err != nil {
                log.Printf("Error approving: %v", err)
                fmt.Fprintf(w, "Error: %v", err)
                return
            }
            fmt.Fprintf(w, "✅ Approved request %s", requestID)

        case "deny":
            if err := warden.Deny(ctx, requestID); err != nil {
                log.Printf("Error denying: %v", err)
                fmt.Fprintf(w, "Error: %v", err)
                return
            }
            fmt.Fprintf(w, "❌ Denied request %s", requestID)
        }
    }
}

// Update main() to include HTTP server
func main() {
    // ... existing code ...

    // Start HTTP server for Slack interactions
    http.HandleFunc("/slack/actions", handleSlackActions(warden))

    go func() {
        log.Println("Starting Slack interaction server on :3000")
        if err := http.ListenAndServe(":3000", nil); err != nil {
            log.Fatal(err)
        }
    }()

    // ... existing polling code ...
}
```

### Build and Run

```bash
# Add Slack SDK to dependencies
go get github.com/slack-go/slack

# Build
go build -o bin/slack-bridge cmd/slack-bridge/main.go

# Run
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_CHANNEL="#clawrden-approvals"
export WARDEN_API_URL="http://localhost:8080"

./bin/slack-bridge
```

### Configure Slack App

1. Go to your app's settings → **Interactivity & Shortcuts**
2. Enable interactivity
3. Set Request URL: `https://your-domain.com/slack/actions`
   - Use ngrok for local testing: `ngrok http 3000`
4. Save changes

---

## Quick Implementation: Telegram Integration

### Prerequisites

1. **Create Telegram Bot**
   - Talk to @BotFather on Telegram
   - `/newbot` and follow instructions
   - Copy Bot Token

2. **Environment Variables**
   ```bash
   export TELEGRAM_BOT_TOKEN="your-bot-token"
   export TELEGRAM_CHAT_ID="your-chat-id"
   export WARDEN_API_URL="http://localhost:8080"
   ```

### Implementation: Telegram Bridge

Create `cmd/telegram-bridge/main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "strconv"
    "strings"
    "time"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    chatID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
    wardenURL := os.Getenv("WARDEN_API_URL")

    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        log.Fatal(err)
    }

    warden := NewWardenClient(wardenURL)
    notified := make(map[string]bool)

    log.Printf("Telegram bridge started. Bot: @%s", bot.Self.UserName)

    // Poll for new HITL requests
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        for range ticker.C {
            items, err := warden.GetQueue(context.Background())
            if err != nil {
                continue
            }

            for _, item := range items {
                if notified[item.ID] {
                    continue
                }

                cmdStr := fmt.Sprintf("%s %s", item.Command,
                    strings.Join(item.Args, " "))

                msg := tgbotapi.NewMessage(chatID,
                    fmt.Sprintf("🔔 *New Command Approval Request*\n\n"+
                        "```\n%s\n```\n"+
                        "📁 Directory: `%s`\n"+
                        "👤 User: `uid:%d`\n"+
                        "🆔 ID: `%s`\n\n"+
                        "Use `/approve %s` or `/deny %s`",
                        cmdStr, item.Cwd, item.Identity.UID,
                        item.ID, item.ID, item.ID))
                msg.ParseMode = "Markdown"

                // Add inline keyboard
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                    tgbotapi.NewInlineKeyboardRow(
                        tgbotapi.NewInlineKeyboardButtonData(
                            "✅ Approve", "approve:"+item.ID),
                        tgbotapi.NewInlineKeyboardButtonData(
                            "❌ Deny", "deny:"+item.ID),
                    ),
                )
                msg.ReplyMarkup = keyboard

                bot.Send(msg)
                notified[item.ID] = true
            }
        }
    }()

    // Handle commands and button clicks
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        // Handle callback queries (button clicks)
        if update.CallbackQuery != nil {
            callback := update.CallbackQuery
            parts := strings.Split(callback.Data, ":")
            if len(parts) != 2 {
                continue
            }

            action, requestID := parts[0], parts[1]

            var err error
            var response string

            switch action {
            case "approve":
                err = warden.Approve(context.Background(), requestID)
                response = fmt.Sprintf("✅ Approved request %s", requestID)
            case "deny":
                err = warden.Deny(context.Background(), requestID)
                response = fmt.Sprintf("❌ Denied request %s", requestID)
            }

            if err != nil {
                response = fmt.Sprintf("Error: %v", err)
            }

            // Answer callback
            bot.Send(tgbotapi.NewCallback(callback.ID, response))

            // Edit original message
            edit := tgbotapi.NewEditMessageText(
                callback.Message.Chat.ID,
                callback.Message.MessageID,
                callback.Message.Text+"\n\n"+response,
            )
            edit.ParseMode = "Markdown"
            bot.Send(edit)

            continue
        }

        // Handle text commands
        if update.Message == nil {
            continue
        }

        if !update.Message.IsCommand() {
            continue
        }

        switch update.Message.Command() {
        case "start":
            msg := tgbotapi.NewMessage(chatID,
                "🛡️ *Clawrden Bot*\n\n"+
                    "I'll notify you of pending approvals.\n\n"+
                    "*Commands:*\n"+
                    "/status - Warden status\n"+
                    "/queue - Pending requests\n"+
                    "/history - Recent commands\n"+
                    "/approve <id> - Approve request\n"+
                    "/deny <id> - Deny request")
            msg.ParseMode = "Markdown"
            bot.Send(msg)

        case "status":
            // TODO: Implement status query
            msg := tgbotapi.NewMessage(chatID, "✅ Warden is running")
            bot.Send(msg)

        case "queue":
            items, _ := warden.GetQueue(context.Background())
            if len(items) == 0 {
                bot.Send(tgbotapi.NewMessage(chatID, "No pending requests"))
            } else {
                for _, item := range items {
                    cmdStr := fmt.Sprintf("%s %s", item.Command,
                        strings.Join(item.Args, " "))
                    msg := tgbotapi.NewMessage(chatID,
                        fmt.Sprintf("🆔 `%s`\n```\n%s\n```", item.ID, cmdStr))
                    msg.ParseMode = "Markdown"
                    bot.Send(msg)
                }
            }

        case "approve":
            args := update.Message.CommandArguments()
            if args == "" {
                bot.Send(tgbotapi.NewMessage(chatID, "Usage: /approve <request-id>"))
                continue
            }
            err := warden.Approve(context.Background(), args)
            if err != nil {
                bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Error: %v", err)))
            } else {
                bot.Send(tgbotapi.NewMessage(chatID, "✅ Approved"))
            }

        case "deny":
            args := update.Message.CommandArguments()
            if args == "" {
                bot.Send(tgbotapi.NewMessage(chatID, "Usage: /deny <request-id>"))
                continue
            }
            err := warden.Deny(context.Background(), args)
            if err != nil {
                bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Error: %v", err)))
            } else {
                bot.Send(tgbotapi.NewMessage(chatID, "❌ Denied"))
            }
        }
    }
}
```

### Build and Run

```bash
# Add Telegram SDK
go get github.com/go-telegram-bot-api/telegram-bot-api/v5

# Build
go build -o bin/telegram-bridge cmd/telegram-bridge/main.go

# Run
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_CHAT_ID="123456789"  # Get from @userinfobot
export WARDEN_API_URL="http://localhost:8080"

./bin/telegram-bridge
```

---

## Deployment

### Docker Compose Integration

Add to `docker-compose.yml`:

```yaml
services:
  warden:
    # ... existing config ...

  slack-bridge:
    build:
      context: .
      dockerfile: docker/Dockerfile.slack-bridge
    environment:
      - SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
      - SLACK_CHANNEL=${SLACK_CHANNEL}
      - WARDEN_API_URL=http://warden:8080
    depends_on:
      - warden
    restart: unless-stopped

  telegram-bridge:
    build:
      context: .
      dockerfile: docker/Dockerfile.telegram-bridge
    environment:
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - TELEGRAM_CHAT_ID=${TELEGRAM_CHAT_ID}
      - WARDEN_API_URL=http://warden:8080
    depends_on:
      - warden
    restart: unless-stopped
```

### Makefile Updates

```makefile
build-slack-bridge:
	go build -o bin/slack-bridge cmd/slack-bridge/main.go

build-telegram-bridge:
	go build -o bin/telegram-bridge cmd/telegram-bridge/main.go

build-all: build build-slack-bridge build-telegram-bridge
```

---

## Security Considerations

1. **Token Security**
   - Never commit tokens to git
   - Use environment variables or secrets management
   - Rotate tokens regularly

2. **Access Control**
   - Restrict chat channels to authorized users
   - Implement approval ACLs (who can approve what)
   - Log all chat-based approvals

3. **Rate Limiting**
   - Prevent spam approvals/denials
   - Implement cooldown periods

4. **Audit Trail**
   - Log chat username alongside approval
   - Track approval source (CLI vs Slack vs Telegram)

---

## Future Enhancements

- **Discord Integration** (similar pattern)
- **Microsoft Teams** connector
- **Multi-platform support** (approve from any chat)
- **Rich notifications** (command previews, risk scores)
- **Slash commands** in chat (`/clawrden status`)
- **WebSocket push** (no polling)
- **Approval workflows** (require 2+ approvers for dangerous commands)

---

## Testing

```bash
# Test Slack bridge
./bin/slack-bridge &

# Trigger a command that needs approval
./bin/clawrden-cli queue  # Should see request

# Check Slack - you should see notification with buttons
# Click approve → request should complete

# Test Telegram bridge
./bin/telegram-bridge &

# Telegram bot should send notifications
# Use /approve <id> or click inline buttons
```

---

**Summary:** Chat integration is straightforward using the existing HTTP API. The bridge services poll the warden queue and post notifications with action buttons. Approvals/denials flow back through the API. Total implementation time: **2-4 hours per platform**.
