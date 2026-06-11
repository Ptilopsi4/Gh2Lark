package lark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// MaxMessageSize is the Lark webhook body size limit (20 KB).
const MaxMessageSize = 20 * 1024

// Client sends messages to a Lark custom bot webhook.
type Client struct {
	webhookURL string
	httpClient *http.Client
}

// TextMessage is a Lark text message payload (msg_type: text).
type TextMessage struct {
	MsgType string      `json:"msg_type"`
	Content TextContent `json:"content"`
}

// TextContent wraps the plain text body.
type TextContent struct {
	Text string `json:"text"`
}

// InteractiveMessage is a Lark interactive card payload (msg_type: interactive).
type InteractiveMessage struct {
	MsgType string     `json:"msg_type"`
	Card    CardConfig `json:"card"`
}

// CardConfig holds the card schema version and visual blocks.
type CardConfig struct {
	Schema string     `json:"schema"`
	Header CardHeader `json:"header,omitempty"`
	Body   CardBody   `json:"body,omitempty"`
}

// CardHeader is the top bar of a card.
type CardHeader struct {
	Title    TextTag `json:"title"`
	Template string  `json:"template,omitempty"` // "blue", "green", "red", "orange", "turquoise", "purple"
}

// CardBody contains the vertical list of card elements.
type CardBody struct {
	Direction string        `json:"direction"`
	Elements  []CardElement `json:"elements"`
}

// CardElement is a single row inside the card body.
// Supported tags: "markdown", "hr", "note", "button".
type CardElement struct {
	Tag       string     `json:"tag"`
	Content   string     `json:"content,omitempty"`
	Text      *TextTag   `json:"text,omitempty"`
	Behaviors []Behavior `json:"behaviors,omitempty"`
}

// Behavior describes a button action. Only "open_url" is supported by custom bots.
type Behavior struct {
	Type       string `json:"type"`
	DefaultURL string `json:"default_url"`
}

// TextTag is a Lark text object used in header titles and button labels.
type TextTag struct {
	Tag     string `json:"tag"` // "plain_text" or "lark_md"
	Content string `json:"content"`
}

// Response is the Lark API response envelope.
type Response struct {
	StatusCode int    `json:"StatusCode"`
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
}

// NewClient creates a Lark webhook client with a 30-second HTTP timeout.
func NewClient(webhookURL string) *Client {
	return &Client{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendText sends a simple text message to the Lark webhook.
func (c *Client) SendText(text string) error {
	msg := TextMessage{
		MsgType: "text",
		Content: TextContent{Text: text},
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal text message: %w", err)
	}
	return c.post(body)
}

// SendCard sends an interactive card message to the Lark webhook.
func (c *Client) SendCard(card *InteractiveMessage) error {
	card.MsgType = "interactive"
	if card.Card.Schema == "" {
		card.Card.Schema = "2.0"
	}
	body, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal card: %w", err)
	}
	return c.post(body)
}

func (c *Client) post(body []byte) error {
	if len(body) > MaxMessageSize {
		return fmt.Errorf("message body size %d exceeds Lark limit of %d bytes", len(body), MaxMessageSize)
	}

	req, err := http.NewRequest(http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("lark webhook request failed", "error", err)
		return fmt.Errorf("send to lark: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("lark webhook returned non-2xx", "status", resp.StatusCode)
		return fmt.Errorf("lark webhook returned HTTP %d", resp.StatusCode)
	}

	var lr Response
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		slog.Warn("failed to decode lark response", "error", err)
		// Non-fatal — we got a 2xx so the message probably went through.
		return nil
	}

	if lr.Code != 0 {
		slog.Error("lark API returned error code", "code", lr.Code, "msg", lr.Msg)
		return fmt.Errorf("lark API error: code=%d msg=%s", lr.Code, lr.Msg)
	}

	return nil
}
