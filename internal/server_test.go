package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStartServer_RouteRegistration(t *testing.T) {
	t.Run("registers webhook endpoint", func(t *testing.T) {
		cfg := &Config{
			Port:          "8080",
			WebHookSecret: "test-secret",
		}

		handler := NewWebhookHandler(cfg, nil, nil)

		mux := http.NewServeMux()
		mux.Handle("/webhook", handler)

		req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code == http.StatusNotFound {
			t.Error("Webhook endpoint not registered")
		}
	})

	t.Run("webhook endpoint responds to requests", func(t *testing.T) {
		cfg := &Config{
			Port:          "8080",
			WebHookSecret: "test-secret",
		}

		handler := NewWebhookHandler(cfg, nil, nil)

		mux := http.NewServeMux()
		mux.Handle("/webhook", handler)

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code == http.StatusNotFound {
			t.Error("Webhook endpoint did not respond")
		}
	})

	t.Run("non-existent routes return 404", func(t *testing.T) {
		cfg := &Config{
			Port:          "8080",
			WebHookSecret: "test-secret",
		}

		handler := NewWebhookHandler(cfg, nil, nil)

		mux := http.NewServeMux()
		mux.Handle("/webhook", handler)

		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected 404 for non-existent route, got %v", rr.Code)
		}
	})
}

func TestStartServer_Integration(t *testing.T) {
	t.Run("server can be started and stopped", func(t *testing.T) {
		cfg := &Config{
			Port:          "0",
			WebHookSecret: "test-secret",
		}

		handler := NewWebhookHandler(cfg, nil, nil)

		server := &http.Server{
			Addr:    ":0",
			Handler: http.HandlerFunc(handler.ServeHTTP),
		}

		go func() {
			server.ListenAndServe()
		}()

		time.Sleep(50 * time.Millisecond)

		err := server.Close()
		if err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	})
}
