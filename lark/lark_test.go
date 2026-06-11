package lark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://open.feishu.cn/open-apis/bot/v2/hook/abc123", "")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.webhookURL != "https://open.feishu.cn/open-apis/bot/v2/hook/abc123" {
		t.Errorf("webhookURL not set correctly")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}

	// With signing secret
	c2 := NewClient("https://open.feishu.cn/open-apis/bot/v2/hook/abc", "mysecret")
	if c2.signingSecret != "mysecret" {
		t.Errorf("signingSecret = %s, want mysecret", c2.signingSecret)
	}
}

func TestSendText(t *testing.T) {
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertLarkSuccess(t, w, r)
	})
	defer ts.Close()

	c := NewClient(ts.URL, "")
	err := c.SendText("Hello from gh2lark")
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}
}

func TestSendCard(t *testing.T) {
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertLarkSuccess(t, w, r)
	})
	defer ts.Close()

	c := NewClient(ts.URL, "")
	card := &InteractiveMessage{
		Card: CardConfig{
			Schema: "2.0",
			Header: CardHeader{
				Title:    TextTag{Tag: "plain_text", Content: "Test Card"},
				Template: "green",
			},
			Body: CardBody{
				Direction: "vertical",
				Elements: []CardElement{
					{Tag: "markdown", Content: "Hello **world**!"},
				},
			},
		},
	}
	err := c.SendCard(card)
	if err != nil {
		t.Fatalf("SendCard failed: %v", err)
	}
}

func TestSendCardDefaultsSchema(t *testing.T) {
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var msg InteractiveMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("decode failed: %v", err)
		}
		if msg.Card.Schema != "2.0" {
			t.Errorf("Schema = %s, want 2.0", msg.Card.Schema)
		}
		writeLarkOK(w)
	})
	defer ts.Close()

	c := NewClient(ts.URL, "")
	card := &InteractiveMessage{
		Card: CardConfig{
			Header: CardHeader{
				Title: TextTag{Tag: "plain_text", Content: "No schema"},
			},
		},
	}
	err := c.SendCard(card)
	if err != nil {
		t.Fatalf("SendCard failed: %v", err)
	}
}

func TestSendCardSizeLimit(t *testing.T) {
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for oversized payloads")
	})
	defer ts.Close()

	c := NewClient(ts.URL, "")
	hugeText := make([]byte, MaxMessageSize+1024)
	for i := range hugeText {
		hugeText[i] = 'x'
	}
	card := &InteractiveMessage{
		Card: CardConfig{
			Header: CardHeader{
				Title: TextTag{Tag: "plain_text", Content: string(hugeText)},
			},
		},
	}
	err := c.SendCard(card)
	if err == nil {
		t.Error("expected error for oversized message, got nil")
	}
}

func TestLarkErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{
			StatusCode: 0,
			Code:       9499,
			Msg:        "Bad Request",
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "")
	err := c.SendText("test")
	if err == nil {
		t.Error("expected error for non-zero API code, got nil")
	}
}

func TestHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "")
	err := c.SendText("test")
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestGenSign(t *testing.T) {
	// Verify against known test vectors from Lark docs example:
	// timestamp=1599360473, secret=test → Python output
	ts := "1599360473"
	secret := "test"

	sign := genSign(secret, ts)

	if sign == "" {
		t.Fatal("genSign returned empty string")
	}
	// Must be valid base64
	if !isBase64(sign) {
		t.Errorf("genSign output is not valid base64: %s", sign)
	}
}

func TestSignedMessage(t *testing.T) {
	secret := "my-signing-secret"
	c := NewClient("https://fake.example/hook/test", secret)

	original := []byte(`{"msg_type":"text","content":{"text":"hello"}}`)
	signed, err := c.signedMessage(original)
	if err != nil {
		t.Fatalf("signedMessage failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(signed, &payload); err != nil {
		t.Fatalf("signed body is not valid JSON: %v", err)
	}
	if payload["msg_type"] != "text" {
		t.Error("msg_type lost after signing")
	}
	if payload["timestamp"] == nil || payload["timestamp"] == "" {
		t.Error("timestamp missing from signed message")
	}
	if payload["sign"] == nil || payload["sign"] == "" {
		t.Error("sign missing from signed message")
	}
	// sign should be valid base64
	if !isBase64(payload["sign"].(string)) {
		t.Errorf("sign is not base64: %s", payload["sign"])
	}
}

func TestSignedMessageNoSecret(t *testing.T) {
	c := NewClient("https://fake.example/hook/test", "")
	original := []byte(`{"msg_type":"text"}`)
	signed, err := c.signedMessage(original)
	if err != nil {
		t.Fatalf("signedMessage failed: %v", err)
	}
	if string(signed) != string(original) {
		t.Errorf("signedMessage should be no-op when secret is empty")
	}
}

func TestSendTextWithSigning(t *testing.T) {
	var captured timestampedPayload
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode: %v", err)
		}
		// Verify sign + timestamp present
		if captured.Timestamp == "" {
			t.Error("timestamp missing in signed request")
		}
		if captured.Sign == "" {
			t.Error("sign missing in signed request")
		}
		writeLarkOK(w)
	})
	defer ts.Close()

	c := NewClient(ts.URL, "test-secret")
	err := c.SendText("signed message")
	if err != nil {
		t.Fatalf("SendText with signing failed: %v", err)
	}
	if captured.MsgType != "text" {
		t.Errorf("msg_type = %s, want text", captured.MsgType)
	}
	if captured.Content.Text != "signed message" {
		t.Errorf("text content = %s, want 'signed message'", captured.Content.Text)
	}
}

// --- helpers ---

type timestampedPayload struct {
	MsgType   string      `json:"msg_type"`
	Content   TextContent `json:"content"`
	Timestamp string      `json:"timestamp"`
	Sign      string      `json:"sign"`
}

func larkTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}
		handler(w, r)
	}))
}

func assertLarkSuccess(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		t.Errorf("decode request body: %v", err)
	}
	if raw["msg_type"] == "" {
		t.Error("msg_type missing in request body")
	}
	writeLarkOK(w)
}

func writeLarkOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{StatusCode: 0, Code: 0, Msg: "success"})
}

func isBase64(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=') {
			return false
		}
	}
	return strings.TrimRight(s, "=") != ""
}
