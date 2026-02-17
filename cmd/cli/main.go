// Command clawrden-cli is the Clawrden control CLI.
// It communicates with the Warden HTTP API to view status,
// manage the HITL queue, and view audit history.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

const version = "1.0.0"

func main() {
	apiURL := flag.String("api", "http://localhost:8080", "Warden API URL")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "clawrden-cli v%s - Clawrden Control Interface\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: clawrden-cli [options] <command>\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  status              Show warden status\n")
		fmt.Fprintf(os.Stderr, "  queue               List pending HITL requests\n")
		fmt.Fprintf(os.Stderr, "  approve <id>        Approve pending request\n")
		fmt.Fprintf(os.Stderr, "  deny <id>           Deny pending request\n")
		fmt.Fprintf(os.Stderr, "  history             View command audit log\n")
		fmt.Fprintf(os.Stderr, "  kill                Trigger kill switch\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	command := flag.Arg(0)
	client := &Client{baseURL: *apiURL}

	switch command {
	case "status":
		if err := client.Status(); err != nil {
			fatal("status: %v", err)
		}
	case "queue":
		if err := client.Queue(); err != nil {
			fatal("queue: %v", err)
		}
	case "approve":
		if flag.NArg() < 2 {
			fatal("approve requires request ID")
		}
		if err := client.Approve(flag.Arg(1)); err != nil {
			fatal("approve: %v", err)
		}
		fmt.Println("Request approved")
	case "deny":
		if flag.NArg() < 2 {
			fatal("deny requires request ID")
		}
		if err := client.Deny(flag.Arg(1)); err != nil {
			fatal("deny: %v", err)
		}
		fmt.Println("Request denied")
	case "history":
		if err := client.History(); err != nil {
			fatal("history: %v", err)
		}
	case "kill":
		if err := client.Kill(); err != nil {
			fatal("kill: %v", err)
		}
		fmt.Println("Kill switch activated")
	default:
		fatal("unknown command: %s", command)
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

// Client is the HTTP client for the Warden API.
type Client struct {
	baseURL string
}

// Status displays the warden status.
func (c *Client) Status() error {
	resp, err := http.Get(c.baseURL + "/api/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	fmt.Printf("Status: %v\n", data["status"])
	fmt.Printf("Pending HITL Requests: %v\n", data["pending_count"])
	return nil
}

// Queue lists pending HITL requests.
func (c *Client) Queue() error {
	resp, err := http.Get(c.baseURL + "/api/queue")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var queue []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&queue); err != nil {
		return err
	}

	if len(queue) == 0 {
		fmt.Println("No pending requests")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCOMMAND\tARGS\tCWD\tUID")
	for _, req := range queue {
		args := ""
		if a, ok := req["args"].([]interface{}); ok {
			parts := make([]string, len(a))
			for i, v := range a {
				parts[i] = fmt.Sprintf("%v", v)
			}
			args = strings.Join(parts, " ")
		}

		identity := req["identity"].(map[string]interface{})
		uid := identity["uid"]

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n",
			req["id"], req["command"], args, req["cwd"], uid)
	}
	w.Flush()
	return nil
}

// Approve approves a pending HITL request.
func (c *Client) Approve(id string) error {
	return c.resolveRequest(id, "approve")
}

// Deny denies a pending HITL request.
func (c *Client) Deny(id string) error {
	return c.resolveRequest(id, "deny")
}

func (c *Client) resolveRequest(id, action string) error {
	url := fmt.Sprintf("%s/api/queue/%s/%s", c.baseURL, id, action)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// History displays the command audit log.
func (c *Client) History() error {
	resp, err := http.Get(c.baseURL + "/api/history")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var history []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return err
	}

	if len(history) == 0 {
		fmt.Println("No audit history")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tCOMMAND\tDECISION\tEXIT\tDURATION")
	for _, entry := range history {
		timestamp := entry["timestamp"].(string)
		// Parse and format timestamp
		t, err := time.Parse(time.RFC3339Nano, timestamp)
		if err == nil {
			timestamp = t.Format("15:04:05")
		}

		duration := ""
		if d, ok := entry["duration_ms"].(float64); ok && d > 0 {
			duration = fmt.Sprintf("%.0fms", d)
		}

		exitCode := ""
		if e, ok := entry["exit_code"].(float64); ok {
			exitCode = fmt.Sprintf("%d", int(e))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			timestamp, entry["command"], entry["decision"], exitCode, duration)
	}
	w.Flush()
	return nil
}

// Kill triggers the kill switch.
func (c *Client) Kill() error {
	resp, err := http.Post(c.baseURL+"/api/kill", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if msg, ok := result["message"]; ok {
		fmt.Printf("Response: %s\n", msg)
	}

	return nil
}
