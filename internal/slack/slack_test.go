package slack

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotify_usesMemberIDWithoutLookup(t *testing.T) {
	var posted map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "lookupByEmail") {
			t.Fatal("should not lookup when member ID set")
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &posted)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "#builds", "U123")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("success", "v-dev-0.0.5", "https://example.com/run", "me@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !mentioned {
		t.Fatal("expected mentioned=true")
	}
	if !strings.Contains(posted["text"], "<@U123>") {
		t.Fatalf("text missing mention: %q", posted["text"])
	}
	if !strings.Contains(posted["text"], "v-dev-0.0.5") {
		t.Fatalf("text missing tag: %q", posted["text"])
	}
	if posted["channel"] != "#builds" {
		t.Fatalf("channel = %q", posted["channel"])
	}
}

func TestNotify_emailLookupFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "lookupByEmail"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":   true,
				"user": map[string]any{"id": "U999"},
			})
		case strings.Contains(r.URL.Path, "chat.postMessage"):
			body, _ := io.ReadAll(r.Body)
			var posted map[string]string
			_ = json.Unmarshal(body, &posted)
			if !strings.Contains(posted["text"], "<@U999>") {
				t.Errorf("expected <@U999>, got %q", posted["text"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "C123", "")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("started", "v-1", "https://example.com", "dev@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !mentioned {
		t.Fatal("expected mentioned=true after email lookup")
	}
}

func TestNotify_noMentionWhenUnresolved(t *testing.T) {
	var posted map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "lookupByEmail") {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "users_not_found"})
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &posted)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := NewClient("xoxb-test", "#builds", "")
	c.baseURL = srv.URL

	mentioned, err := c.Notify("failure", "v-1", "https://example.com", "missing@example.com")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if mentioned {
		t.Fatal("expected mentioned=false")
	}
	if strings.Contains(posted["text"], "<@") {
		t.Fatalf("should not mention: %q", posted["text"])
	}
}

func TestFormatMessage(t *testing.T) {
	cases := []struct {
		event string
		want  string
	}{
		{"started", "started"},
		{"success", "successfully"},
		{"failure", "failed"},
		{"cancelled", "cancelled"},
	}
	for _, tc := range cases {
		msg := formatMessage(tc.event, "v-1", "U1", "https://x")
		if !strings.Contains(msg, tc.want) || !strings.Contains(msg, "<@U1>") {
			t.Fatalf("event %s => %q", tc.event, msg)
		}
	}
}
