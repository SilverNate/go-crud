package httpdelivery

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-crud/go-task-api/internal/domain"
	"github.com/go-crud/go-task-api/internal/repository/memory"
	"github.com/go-crud/go-task-api/internal/usecase"
)

// newServer wires the REAL router, use case and in-memory repository, so these
// tests exercise the whole HTTP -> use case -> repository path.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := usecase.NewTask(memory.New(), nil, log)
	srv := httptest.NewServer(NewRouter(svc, log))
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestHealth(t *testing.T) {
	srv := newServer(t)
	resp, body := call(t, http.MethodGet, srv.URL+"/api/v1/health", "")
	if resp.StatusCode != http.StatusOK || body != "ok" {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
}

func TestRequestLoggerRecordsRequest(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	svc := usecase.NewTask(memory.New(), nil, log)
	h := NewRouter(svc, log)

	// Direct (synchronous) call so the log line is complete when we read it.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/7", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log output is not JSON: %v (%q)", err, buf.String())
	}
	if line["msg"] != "HTTP request" || line["method"] != "GET" || line["path"] != "/api/v1/tasks/7" {
		t.Fatalf("log line = %v", line)
	}
	if line["status"] != float64(http.StatusNotFound) {
		t.Fatalf("logged status = %v, want 404", line["status"])
	}
	if _, ok := line["duration"]; !ok {
		t.Fatal("duration missing")
	}
	if id, _ := line["request_id"].(string); id == "" {
		t.Fatal("request_id missing from log line")
	}
}

func TestFullCRUDFlow(t *testing.T) {
	srv := newServer(t)
	base := srv.URL + "/api/v1/tasks"

	// Empty list is [] not null.
	resp, body := call(t, http.MethodGet, base, "")
	if resp.StatusCode != 200 || strings.TrimSpace(body) != "[]" {
		t.Fatalf("empty list: %d %q", resp.StatusCode, body)
	}

	// Create.
	resp, body = call(t, http.MethodPost, base, `{"title":"  Write tests ","description":"all layers"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created domain.Task
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Title != "Write tests" || created.Completed {
		t.Fatalf("created = %+v", created)
	}

	// Get.
	resp, body = call(t, http.MethodGet, base+"/1", "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"Write tests"`) {
		t.Fatalf("get: %d %s", resp.StatusCode, body)
	}

	// Update.
	resp, body = call(t, http.MethodPut, base+"/1", `{"title":"Done","description":"x","completed":true}`)
	var updated domain.Task
	_ = json.Unmarshal([]byte(body), &updated)
	if resp.StatusCode != 200 || updated.Title != "Done" || !updated.Completed {
		t.Fatalf("update: %d %s", resp.StatusCode, body)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) || !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("timestamps wrong: created=%v updated=%v", updated.CreatedAt, updated.UpdatedAt)
	}

	// Partial update without "completed" keeps it true.
	_, body = call(t, http.MethodPut, base+"/1", `{"title":"Done again"}`)
	_ = json.Unmarshal([]byte(body), &updated)
	if !updated.Completed {
		t.Fatal("omitted completed must preserve the stored value")
	}

	// List shows one.
	_, body = call(t, http.MethodGet, base, "")
	var list []domain.Task
	_ = json.Unmarshal([]byte(body), &list)
	if len(list) != 1 {
		t.Fatalf("list = %s", body)
	}

	// Delete, then it is gone.
	resp, _ = call(t, http.MethodDelete, base+"/1", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = call(t, http.MethodGet, base+"/1", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: %d", resp.StatusCode)
	}
	resp, _ = call(t, http.MethodDelete, base+"/1", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete: %d", resp.StatusCode)
	}
}

func TestValidationErrorsThroughFullStack(t *testing.T) {
	srv := newServer(t)
	base := srv.URL + "/api/v1/tasks"
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"create blank title", "POST", base, `{"title":"   "}`, 400},
		{"create missing title", "POST", base, `{}`, 400},
		{"create invalid json", "POST", base, `{`, 400},
		{"get non-numeric id", "GET", base + "/abc", "", 400},
		{"get missing", "GET", base + "/42", "", 404},
		{"put missing", "PUT", base + "/42", `{"title":"x"}`, 404},
		{"put blank title", "PUT", base + "/42", `{"title":""}`, 404}, // 404 first: existence is checked before validation
		{"delete missing", "DELETE", base + "/42", "", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := call(t, tt.method, tt.path, tt.body)
			if resp.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d (%s)", resp.StatusCode, tt.want, body)
			}
			if tt.want >= 400 {
				var e map[string]string
				if err := json.Unmarshal([]byte(body), &e); err != nil || e["error"] == "" {
					t.Fatalf("error body not in {error: ...} form: %q", body)
				}
			}
		})
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	srv := newServer(t)
	tests := []struct {
		name, method, path string
		want               int
	}{
		{"unknown path", "GET", "/nope", 404},
		{"unversioned path", "GET", "/tasks", 404},
		{"unknown nested path", "GET", "/api/v1/unknown", 404},
		{"too deep path", "GET", "/api/v1/tasks/1/extra", 404},
		{"PATCH not allowed", "PATCH", "/api/v1/tasks/1", 405},
		{"DELETE on collection not allowed", "DELETE", "/api/v1/tasks", 405},
		{"POST on item not allowed", "POST", "/api/v1/tasks/1", 405},
		{"POST on health not allowed", "POST", "/api/v1/health", 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := call(t, tt.method, srv.URL+tt.path, "")
			if resp.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}
