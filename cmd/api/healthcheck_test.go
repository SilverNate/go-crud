package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunHealthcheck(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/health" {
				t.Errorf("probed %q", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		if err := runHealthcheck(srv.Listener.Addr().String(), time.Second); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unhealthy status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if err := runHealthcheck(srv.Listener.Addr().String(), time.Second); err == nil {
			t.Fatal("want error for 503")
		}
	})
	t.Run("nothing listening", func(t *testing.T) {
		if err := runHealthcheck("127.0.0.1:1", time.Second); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("invalid address", func(t *testing.T) {
		if err := runHealthcheck("not-an-addr", time.Second); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("wildcard address probes loopback", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		defer srv.Close()
		_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
		if err := runHealthcheck(":"+port, time.Second); err != nil {
			t.Fatal(err)
		}
	})
}
