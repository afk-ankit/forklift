package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	token    string
	channel  string
	memberID string
	baseURL  string
	http     *http.Client
}

func NewClient(token, channel, memberID string) *Client {
	return &Client{
		token:    token,
		channel:  channel,
		memberID: memberID,
		baseURL:  "https://slack.com/api",
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify posts a channel message for a poll event. gitEmail is used only when memberID is empty.
func (c *Client) Notify(event, tag, htmlURL, gitEmail string) (bool, error) {
	userID := strings.TrimSpace(c.memberID)
	if userID == "" && gitEmail != "" {
		id, err := c.lookupByEmail(gitEmail)
		if err == nil {
			userID = id
		}
	}
	text := formatMessage(event, tag, userID, htmlURL)
	if err := c.postMessage(text); err != nil {
		return false, err
	}
	return userID != "", nil
}

func formatMessage(event, tag, userID, htmlURL string) string {
	mention := ""
	if userID != "" {
		mention = " — <@" + userID + ">"
	}
	link := ""
	if htmlURL != "" {
		link = " " + htmlURL
	}
	switch event {
	case "started":
		return fmt.Sprintf("🚧 Forklift: workflow started for tag %s%s%s", tag, mention, link)
	case "success":
		return fmt.Sprintf("✅ Forklift: tag %s built successfully%s%s", tag, mention, link)
	case "failure":
		return fmt.Sprintf("❌ Forklift: tag %s build failed%s%s", tag, mention, link)
	case "cancelled":
		return fmt.Sprintf("⚠️ Forklift: tag %s build cancelled%s%s", tag, mention, link)
	default:
		return fmt.Sprintf("Forklift: tag %s %s%s%s", tag, event, mention, link)
	}
}

func (c *Client) lookupByEmail(email string) (string, error) {
	u := c.baseURL + "/users.lookupByEmail?email=" + url.QueryEscape(email)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("slack lookupByEmail HTTP status %d: %s",
			resp.StatusCode, responseSnippet(resp.Body))
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("slack lookupByEmail: %s", result.Error)
	}
	return result.User.ID, nil
}

func (c *Client) postMessage(text string) error {
	payload, _ := json.Marshal(map[string]string{
		"channel": c.channel,
		"text":    text,
	})
	req, err := http.NewRequest("POST", c.baseURL+"/chat.postMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack chat.postMessage HTTP status %d: %s",
			resp.StatusCode, responseSnippet(bytes.NewReader(body)))
	}

	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("slack chat.postMessage: %s", result.Error)
	}
	return nil
}

func responseSnippet(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, 512))
	return strings.TrimSpace(string(body))
}
