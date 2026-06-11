package lark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://open.feishu.cn/open-apis/bot/v2/hook/abc123")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.webhookURL != "https://open.feishu.cn/open-apis/bot/v2/hook/abc123" {
		t.Errorf("webhookURL not set correctly")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestSendText(t *testing.T) {
	ts := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertLarkSuccess(t, w, r)
	})
	defer ts.Close()

	c := NewClient(ts.URL)
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

	c := NewClient(ts.URL)
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

	c := NewClient(ts.URL)
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

	c := NewClient(ts.URL)
	// Create a message body that exceeds 20KB
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

	c := NewClient(ts.URL)
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

	c := NewClient(ts.URL)
	err := c.SendText("test")
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

// --- helpers ---

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
