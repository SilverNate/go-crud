package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// runHealthcheck probes the local /api/v1/health endpoint. It exists so a
// minimal (distroless) image, which has no curl/wget/shell, can still define a
// Docker HEALTHCHECK: `/app/api healthcheck`.
func runHealthcheck(addr string, timeout time.Duration) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid HTTP_ADDR %q: %w", addr, err)
	}
	// ":8080", "0.0.0.0:8080" and "[::]:8080" listen on all interfaces; probe loopback.
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/api/v1/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}
	return nil
}
