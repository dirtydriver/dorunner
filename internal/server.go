package internal

import (
	"log"
	"net/http"
)

// StartServer registers the webhook endpoint and blocks serving HTTP on the
// port specified in cfg. Exits the process if the listener fails.
func StartServer(cfg *Config, wbh *WebhookHandler) {

	mux := http.NewServeMux()
	mux.Handle("/webhook", wbh)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	err := http.ListenAndServe(":"+cfg.Port, mux)
	if err != nil {
		log.Fatalf("server stopped: %v", err)
	}

}
