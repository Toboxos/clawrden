package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Channel string             `json:"channel"`
	Text    string             `json:"text"`
	Blocks  []SlackBlock       `json:"blocks,omitempty"`
	Actions []SlackAction      `json:"attachments,omitempty"`
}

// SlackBlock represents a Slack block
type SlackBlock struct {
	Type string           `json:"type"`
	Text *SlackTextObject `json:"text,omitempty"`
}

// SlackTextObject represents text in a Slack block
type SlackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackAction represents Slack interactive buttons
type SlackAction struct {
	Text       string        `json:"text"`
	Fallback   string        `json:"fallback"`
	CallbackID string        `json:"callback_id"`
	Color      string        `json:"color"`
	Actions    []SlackButton `json:"actions"`
}

// SlackButton represents a button in Slack
type SlackButton struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Type  string `json:"type"`
	Value string `json:"value"`
	Style string `json:"style,omitempty"`
}

// Simple Slack webhook client (without SDK to avoid dependencies)
func postToSlack(webhookURL, message string) error {
	payload := map[string]string{"text": message}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

func main() {
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	wardenURL := os.Getenv("WARDEN_API_URL")

	if webhookURL == "" {
		log.Fatal("SLACK_WEBHOOK_URL environment variable is required")
	}

	if wardenURL == "" {
		wardenURL = "http://localhost:8080"
	}

	warden := NewWardenClient(wardenURL)
	notified := make(map[string]bool)

	log.Printf("Slack bridge started. Polling warden at %s every 5 seconds...", wardenURL)

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

			// Send notification to Slack
			message := fmt.Sprintf(
				"🔔 *New Command Approval Request*\n\n"+
					"```%s```\n"+
					"📁 Directory: `%s`\n"+
					"👤 User: `uid:%d`\n"+
					"🆔 Request ID: `%s`\n\n"+
					"To approve: `./bin/clawrden-cli approve %s`\n"+
					"To deny: `./bin/clawrden-cli deny %s`\n"+
					"Or visit: http://localhost:8080",
				cmdStr, item.Cwd, item.Identity.UID, item.ID, item.ID, item.ID,
			)

			if err := postToSlack(webhookURL, message); err != nil {
				log.Printf("Error posting to Slack: %v", err)
				continue
			}

			log.Printf("Notified Slack about request %s: %s", item.ID, cmdStr)
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
