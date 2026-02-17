package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// WardenClient communicates with the Clawrden warden API
type WardenClient struct {
	baseURL string
	client  *http.Client
}

// QueueItem represents a pending HITL request
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

// NewWardenClient creates a new warden API client
func NewWardenClient(baseURL string) *WardenClient {
	return &WardenClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetQueue fetches pending HITL requests
func (w *WardenClient) GetQueue(ctx context.Context) ([]QueueItem, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", w.baseURL+"/api/queue", nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var items []QueueItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// Approve approves a pending request
func (w *WardenClient) Approve(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/api/queue/%s/approve", w.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("approve failed with status %d", resp.StatusCode)
	}
	return nil
}

// Deny denies a pending request
func (w *WardenClient) Deny(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/api/queue/%s/deny", w.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deny failed with status %d", resp.StatusCode)
	}
	return nil
}

// Simple Telegram Bot API client (without SDK to avoid dependencies)
func sendTelegramMessage(botToken, chatID, message string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	data, _ := json.Marshal(payload)

	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	wardenURL := os.Getenv("WARDEN_API_URL")

	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	if chatID == "" {
		log.Fatal("TELEGRAM_CHAT_ID environment variable is required")
	}

	if wardenURL == "" {
		wardenURL = "http://localhost:8080"
	}

	warden := NewWardenClient(wardenURL)
	notified := make(map[string]bool)

	log.Printf("Telegram bridge started. Polling warden at %s every 5 seconds...", wardenURL)

	// Send startup message
	startMsg := "🛡️ *Clawrden Telegram Bot Started*\n\n" +
		"I'll notify you of pending command approvals.\n\n" +
		"Use the CLI to approve/deny:\n" +
		"`./bin/clawrden-cli approve <id>`\n" +
		"`./bin/clawrden-cli deny <id>`"

	_ = sendTelegramMessage(botToken, chatID, startMsg)

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
			cmdStr := item.Command
			if len(item.Args) > 0 {
				cmdStr = fmt.Sprintf("%s %s", item.Command, strings.Join(item.Args, " "))
			}

			// Escape special Markdown characters
			cmdStr = strings.ReplaceAll(cmdStr, "_", "\\_")
			cmdStr = strings.ReplaceAll(cmdStr, "*", "\\*")
			cmdStr = strings.ReplaceAll(cmdStr, "[", "\\[")
			cmdStr = strings.ReplaceAll(cmdStr, "`", "\\`")

			// Send notification to Telegram
			message := fmt.Sprintf(
				"🔔 *New Command Approval Request*\n\n"+
					"```\n%s\n```\n"+
					"📁 Directory: `%s`\n"+
					"👤 User: `uid:%d`\n"+
					"🆔 ID: `%s`\n\n"+
					"*To approve:*\n"+
					"`./bin/clawrden-cli approve %s`\n\n"+
					"*To deny:*\n"+
					"`./bin/clawrden-cli deny %s`\n\n"+
					"Or visit: http://localhost:8080",
				url.PathEscape(cmdStr), item.Cwd, item.Identity.UID, item.ID, item.ID, item.ID,
			)

			if err := sendTelegramMessage(botToken, chatID, message); err != nil {
				log.Printf("Error sending to Telegram: %v", err)
				continue
			}

			log.Printf("Notified Telegram about request %s: %s", item.ID, cmdStr)
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
