package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-crud/go-task-api/internal/domain"
)

// stubService lets each test script the service's answer and inspect its input.
type stubService struct {
	task  domain.Task
	tasks []domain.Task
	err   error

	gotTitle, gotDesc string
	gotID             int64
	gotCompleted      *bool
	called            bool
}

func (s *stubService) Create(_ context.Context, title, desc string) (domain.Task, error) {
	s.called, s.gotTitle, s.gotDesc = true, title, desc
	return s.task, s.err
}
func (s *stubService) List(context.Context) ([]domain.Task, error) {
	s.called = true
	return s.tasks, s.err
}
func (s *stubService) Get(_ context.Context, id int64) (domain.Task, error) {
	s.called, s.gotID = true, id
	return s.task, s.err
}
func (s *stubService) Update(_ context.Context, id int64, title, desc string, completed *bool) (domain.Task, error) {
	s.called, s.gotID, s.gotTitle, s.gotDesc, s.gotCompleted = true, id, title, desc, completed
	return s.task, s.err
}
func (s *stubService) Delete(_ context.Context, id int64) error {
	s.called, s.gotID = true, id
	return s.err
}

// newRouter mounts the handler on the same paths as the production router.
func newRouter(svc TaskService) http.Handler {
	h := NewTaskHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Route("/api/v1/tasks", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})
	return r
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, wantMsg string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error body is not JSON: %v (%s)", err, rec.Body)
	}
	if len(resp) != 1 || resp["error"] == "" {
		t.Fatalf("error envelope must be exactly {\"error\": \"...\"}, got %v", resp)
	}
	if wantMsg != "" && resp["error"] != wantMsg {
		t.Fatalf("error = %q, want %q", resp["error"], wantMsg)
	}
}

var sample = domain.Task{ID: 1, Title: "Learn Go", Description: "d", CreatedAt: time.Unix(1700000000, 0).UTC(), UpdatedAt: time.Unix(1700000000, 0).UTC()}

func TestCreate(t *testing.T) {
	t.Run("201 with task JSON", func(t *testing.T) {
		svc := &stubService{task: sample}
		rec := do(t, newRouter(svc), http.MethodPost, "/api/v1/tasks", `{"title":"Learn Go","description":"d"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q", ct)
		}
		var got domain.Task
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != 1 || got.Title != "Learn Go" {
			t.Fatalf("got %+v", got)
		}
		if svc.gotTitle != "Learn Go" || svc.gotDesc != "d" {
			t.Fatalf("service got %q/%q", svc.gotTitle, svc.gotDesc)
		}
	})

	badBodies := map[string]string{
		"malformed JSON":        `{"title":`,
		"empty body":            ``,
		"not an object":         `"just a string"`,
		"wrong field type":      `{"title": 123}`,
		"unknown field":         `{"title":"x","bogus":1}`,
		"two JSON values":       `{"title":"x"}{"title":"y"}`,
		"trailing garbage":      `{"title":"x"} junk`,
		"oversized body (>1MB)": `{"title":"` + strings.Repeat("a", 1<<20+10) + `"}`,
	}
	for name, body := range badBodies {
		t.Run("400 "+name, func(t *testing.T) {
			svc := &stubService{}
			rec := do(t, newRouter(svc), http.MethodPost, "/api/v1/tasks", body)
			assertError(t, rec, http.StatusBadRequest, "invalid JSON body")
			if svc.called {
				t.Fatal("service must not be called for malformed input")
			}
		})
	}

	t.Run("400 empty title (domain validation)", func(t *testing.T) {
		svc := &stubService{err: domain.ErrInvalidTitle}
		rec := do(t, newRouter(svc), http.MethodPost, "/api/v1/tasks", `{"title":"  "}`)
		assertError(t, rec, http.StatusBadRequest, domain.ErrInvalidTitle.Error())
	})

	t.Run("500 hides internal error details", func(t *testing.T) {
		svc := &stubService{err: errors.New("pq: password authentication failed for user admin")}
		rec := do(t, newRouter(svc), http.MethodPost, "/api/v1/tasks", `{"title":"x"}`)
		assertError(t, rec, http.StatusInternalServerError, "internal server error")
		if strings.Contains(rec.Body.String(), "password") {
			t.Fatal("internal error leaked to client")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("200 with tasks", func(t *testing.T) {
		svc := &stubService{tasks: []domain.Task{sample, {ID: 2, Title: "b"}}}
		rec := do(t, newRouter(svc), http.MethodGet, "/api/v1/tasks", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		var got []domain.Task
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 2 {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("200 with empty array", func(t *testing.T) {
		svc := &stubService{tasks: []domain.Task{}}
		rec := do(t, newRouter(svc), http.MethodGet, "/api/v1/tasks", "")
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("body = %q, want []", rec.Body)
		}
	})
	t.Run("500", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: errors.New("db down")}), http.MethodGet, "/api/v1/tasks", "")
		assertError(t, rec, http.StatusInternalServerError, "internal server error")
	})
}

func TestGet(t *testing.T) {
	t.Run("200", func(t *testing.T) {
		svc := &stubService{task: sample}
		rec := do(t, newRouter(svc), http.MethodGet, "/api/v1/tasks/1", "")
		if rec.Code != http.StatusOK || svc.gotID != 1 {
			t.Fatalf("status=%d id=%d", rec.Code, svc.gotID)
		}
		var got domain.Task
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got.Title != "Learn Go" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("404 missing task", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: domain.ErrNotFound}), http.MethodGet, "/api/v1/tasks/99", "")
		assertError(t, rec, http.StatusNotFound, domain.ErrNotFound.Error())
	})
	t.Run("500", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: errors.New("x")}), http.MethodGet, "/api/v1/tasks/1", "")
		assertError(t, rec, http.StatusInternalServerError, "internal server error")
	})
	for _, id := range []string{"abc", "0", "-1", "1.5", "99999999999999999999"} {
		t.Run("400 invalid id "+id, func(t *testing.T) {
			svc := &stubService{}
			rec := do(t, newRouter(svc), http.MethodGet, "/api/v1/tasks/"+id, "")
			assertError(t, rec, http.StatusBadRequest, "id must be a positive integer")
			if svc.called {
				t.Fatal("service must not be called with an invalid id")
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Run("200 passes all fields including completed", func(t *testing.T) {
		svc := &stubService{task: sample}
		rec := do(t, newRouter(svc), http.MethodPut, "/api/v1/tasks/1", `{"title":"New","description":"nd","completed":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body)
		}
		if svc.gotID != 1 || svc.gotTitle != "New" || svc.gotDesc != "nd" || svc.gotCompleted == nil || !*svc.gotCompleted {
			t.Fatalf("service got id=%d %q %q %v", svc.gotID, svc.gotTitle, svc.gotDesc, svc.gotCompleted)
		}
	})
	t.Run("omitted completed is passed as nil", func(t *testing.T) {
		svc := &stubService{task: sample}
		do(t, newRouter(svc), http.MethodPut, "/api/v1/tasks/1", `{"title":"New"}`)
		if svc.gotCompleted != nil {
			t.Fatal("completed must be nil when omitted so existing value is preserved")
		}
	})
	t.Run("400 invalid JSON", func(t *testing.T) {
		svc := &stubService{}
		rec := do(t, newRouter(svc), http.MethodPut, "/api/v1/tasks/1", `{bad`)
		assertError(t, rec, http.StatusBadRequest, "invalid JSON body")
		if svc.called {
			t.Fatal("service must not be called")
		}
	})
	t.Run("400 empty title", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: domain.ErrInvalidTitle}), http.MethodPut, "/api/v1/tasks/1", `{"title":""}`)
		assertError(t, rec, http.StatusBadRequest, domain.ErrInvalidTitle.Error())
	})
	t.Run("400 invalid id", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{}), http.MethodPut, "/api/v1/tasks/abc", `{"title":"x"}`)
		assertError(t, rec, http.StatusBadRequest, "id must be a positive integer")
	})
	t.Run("404", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: domain.ErrNotFound}), http.MethodPut, "/api/v1/tasks/9", `{"title":"x"}`)
		assertError(t, rec, http.StatusNotFound, domain.ErrNotFound.Error())
	})
	t.Run("500", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: errors.New("x")}), http.MethodPut, "/api/v1/tasks/1", `{"title":"x"}`)
		assertError(t, rec, http.StatusInternalServerError, "internal server error")
	})
}

func TestDelete(t *testing.T) {
	t.Run("204 with empty body", func(t *testing.T) {
		svc := &stubService{}
		rec := do(t, newRouter(svc), http.MethodDelete, "/api/v1/tasks/3", "")
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || svc.gotID != 3 {
			t.Fatalf("status=%d body=%q id=%d", rec.Code, rec.Body, svc.gotID)
		}
	})
	t.Run("404 missing", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: domain.ErrNotFound}), http.MethodDelete, "/api/v1/tasks/3", "")
		assertError(t, rec, http.StatusNotFound, domain.ErrNotFound.Error())
	})
	t.Run("400 invalid id", func(t *testing.T) {
		svc := &stubService{}
		rec := do(t, newRouter(svc), http.MethodDelete, "/api/v1/tasks/-4", "")
		assertError(t, rec, http.StatusBadRequest, "id must be a positive integer")
		if svc.called {
			t.Fatal("service must not be called")
		}
	})
	t.Run("500", func(t *testing.T) {
		rec := do(t, newRouter(&stubService{err: errors.New("x")}), http.MethodDelete, "/api/v1/tasks/1", "")
		assertError(t, rec, http.StatusInternalServerError, "internal server error")
	})
}
