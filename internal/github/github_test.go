package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCheckWorkflowStatusForTag_usesBranchQuery(t *testing.T) {
	var gotPath, gotRawQuery string
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{
					"id":          int64(42),
					"status":      "in_progress",
					"conclusion":  nil,
					"html_url":    "https://example.com/run/42",
					"head_branch": "v-dev-0.0.5",
					"created_at":  "2026-07-31T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("token", "org", "repo")
	c.baseURL = srv.URL

	st, err := c.CheckWorkflowStatusForTag("v-dev-0.0.5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.RunID != 42 || st.Status != "in_progress" {
		t.Fatalf("unexpected status: %+v", st)
	}
	if !strings.Contains(gotPath, "/repos/org/repo/actions/runs") {
		t.Fatalf("path = %s", gotPath)
	}
	q, _ := url.ParseQuery(gotRawQuery)
	if q.Get("branch") != "v-dev-0.0.5" {
		t.Fatalf("branch query = %q", q.Get("branch"))
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestCheckWorkflowStatusForTag_fallsBackToPushRuns(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		if requests == 1 {
			if q.Get("branch") != "v-dev-0.0.5" {
				t.Fatalf("branch query = %q", q.Get("branch"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []any{}})
			return
		}
		if q.Get("event") != "push" || q.Get("per_page") != "30" {
			t.Fatalf("fallback query = %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{"id": int64(41), "status": "completed", "head_branch": "other"},
				{"id": int64(42), "status": "in_progress", "head_branch": "v-dev-0.0.5"},
				{"id": int64(43), "status": "queued", "head_branch": "v-dev-0.0.5"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("", "org", "repo")
	c.baseURL = srv.URL

	st, err := c.CheckWorkflowStatusForTag("v-dev-0.0.5")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.RunID != 42 {
		t.Fatalf("run ID = %d, want first matching run 42", st.RunID)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestCheckWorkflowStatusForTag_notFoundAfterFallback(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []any{}})
	}))
	defer srv.Close()

	c := NewClient("", "org", "repo")
	c.baseURL = srv.URL

	_, err := c.CheckWorkflowStatusForTag("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestCheckWorkflowStatusForTag_apiErrorIsFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	c := NewClient("bad", "org", "repo")
	c.baseURL = srv.URL

	_, err := c.CheckWorkflowStatusForTag("v-1")
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("want fatal API error, got %v", err)
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "GitHub API error") {
		t.Fatalf("want API error message, got %v", err)
	}
}

func TestGetWorkflowRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/org/repo/actions/runs/99" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         int64(99),
			"status":     "completed",
			"conclusion": "success",
			"html_url":   "https://example.com/run/99",
			"created_at": "2026-07-31T10:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewClient("t", "org", "repo")
	c.baseURL = srv.URL

	st, err := c.GetWorkflowRun(99)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.Status != "completed" || st.Conclusion != "success" || st.RunID != 99 {
		t.Fatalf("unexpected: %+v", st)
	}
}
