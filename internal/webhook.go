package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type WebhookPayload struct {
	Action      string      `json:"action"`
	Repository  Repository  `json:"repository"`
	WorkflowJob WorkflowJob `json:"workflow_job"`
}

type WorkflowJob struct {
	Labels     []string `json:"labels"`
	ID         int64    `json:"id"`
	RunnerName string   `json:"runner_name"`
}

type Repository struct {
	FullName string `json:"full_name"`
}

type WebhookHandler struct {
	cfg    *Config
	do     *DOClient
	github *GitHubClient
}

func NewWebhookHandler(cfg *Config, do *DOClient, github *GitHubClient) *WebhookHandler {
	return &WebhookHandler{
		cfg:    cfg,
		do:     do,
		github: github,
	}
}
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")

	if sig == "" {
		http.Error(w, "Missing signature", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	if !verifySignature(body, sig, h.cfg.WebHookSecret) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	var payLoad WebhookPayload

	err = json.Unmarshal(body, &payLoad)
	if err != nil {
		http.Error(w, "Failed to unmarshal request body", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	go func() {
		parts := strings.SplitN(payLoad.Repository.FullName, "/", 2)
		repoOwner, repoName := parts[0], parts[1]
		ctx := context.Background()
		switch payLoad.Action {
		case "queued":

			runnerName := fmt.Sprintf("do-runner-%d", payLoad.WorkflowJob.ID)
			log.Printf("Runner name: %s", runnerName)
			jitConfig, err := h.github.FetchJITConfig(ctx, repoOwner, repoName, runnerName, h.cfg.GitHubRunnerGroupID, h.cfg.GitHubRunnerLabels)
			if err != nil {
				log.Printf("failed to fetch JIT config: %v", err)
				return
			}
			err = h.do.CreateDroplet(ctx, runnerName, jitConfig, payLoad.WorkflowJob.Labels)
			if err != nil {
				log.Printf("failed to create droplet: %v", err)
				return
			}

		case "completed":
			runnerName := payLoad.WorkflowJob.RunnerName
			err := h.do.DeleteDroplet(ctx, runnerName)
			if err != nil {
				log.Printf("failed to delete droplet: %v", err)
				return
			}

		default:
			return
		}
	}()

}

func verifySignature(body []byte, signature string, secret string) bool {

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}
