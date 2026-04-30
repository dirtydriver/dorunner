package internal

import (
	"log"
	"net/http"
)

func StartServer(cfg *Config, wbh *WebhookHandler) {

	mux := http.NewServeMux()
	mux.Handle("/webhook", wbh)
	err := http.ListenAndServe(":"+cfg.Port, mux)
	if err != nil {
		log.Fatalf("server stopped: %v", err)
	}

}
