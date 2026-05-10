package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// StartServer registers the webhook endpoint and blocks serving HTTP on the
// port specified in cfg. Exits the process if the listener fails.
func StartServer(cfg *Config, wbh *WebhookHandler) {

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	mux.Handle("/webhook", wbh)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	})

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
		log.Println("waiting for in-flight jobs to complete...")
		wbh.Wait()
		log.Println("all jobs complete, exiting")
	}()

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}

}
