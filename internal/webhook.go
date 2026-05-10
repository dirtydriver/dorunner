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

// WebhookPayload is the subset of the GitHub workflow_job webhook event body
// that dorunner needs to act on.
type WebhookPayload struct {
	Action      string      `json:"action"`
	Repository  Repository  `json:"repository"`
	WorkflowJob WorkflowJob `json:"workflow_job"`
}

// WorkflowJob carries the job metadata included in a workflow_job event.
type WorkflowJob struct {
	Labels     []string `json:"labels"`
	ID         int64    `json:"id"`
	RunnerName string   `json:"runner_name"`
}

// Repository holds the repository context from a webhook event.
type Repository struct {
	FullName string `json:"full_name"` // "owner/repo"
}

// WebhookHandler handles incoming GitHub workflow_job webhook events. It
// creates a DigitalOcean droplet when a job is queued and destroys it when
// the job completes.
type WebhookHandler struct {
	cfg    *Config
	do     *DOClient
	github *GitHubClient
}

// NewWebhookHandler wires together the config, DigitalOcean client, and GitHub
// client needed to handle webhook events.
func NewWebhookHandler(cfg *Config, do *DOClient, github *GitHubClient) *WebhookHandler {
	return &WebhookHandler{
		cfg:    cfg,
		do:     do,
		github: github,
	}
}

// ServeHTTP validates the HMAC-SHA256 signature on the incoming request, then
// dispatches the event asynchronously:
//   - "queued"    → fetch JIT config from GitHub, create a new droplet
//   - "completed" → delete the droplet by runner name
//
// The response is written before the goroutine starts so GitHub's webhook
// delivery does not time out waiting for droplet operations.
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
			// ignore jobs that don't need a self-hosted runner
			if !containsLabel(payLoad.WorkflowJob.Labels, "self-hosted") {
				log.Printf("job %d is not for a self-hosted runner, ignoring", payLoad.WorkflowJob.ID)
				return
			}
			runnerName := fmt.Sprintf("do-runner-%d", payLoad.WorkflowJob.ID)
			log.Printf("[%s] queued — fetching JIT config", runnerName)
			jitConfig, err := h.github.FetchJITConfig(ctx, repoOwner, repoName, runnerName, h.cfg.GitHubRunnerGroupID, payLoad.WorkflowJob.Labels)
			if err != nil {
				log.Printf("failed to fetch JIT config: %v", err)
				return
			}
			log.Printf("[%s] JIT config fetched — creating droplet", runnerName)

			err = h.do.CreateDroplet(ctx, runnerName, jitConfig, payLoad.WorkflowJob.Labels)
			if err != nil {
				log.Printf("failed to create droplet: %v", err)
				return
			}
			log.Printf("[%s] droplet created successfully", runnerName)

		case "completed":
			runnerName := payLoad.WorkflowJob.RunnerName
			if runnerName == "" || !strings.HasPrefix(runnerName, "do-runner-") {
				log.Printf("job %d completed on non-dorunner runner, skipping", payLoad.WorkflowJob.ID)
				return
			}
			err := h.do.DeleteDroplet(ctx, runnerName)
			if err != nil {
				log.Printf("[%s] failed to delete droplet: %v", runnerName, err)
				return
			}
			log.Printf("[%s] droplet deleted successfully", runnerName)

		default:
			return
		}
	}()

}
func containsLabel(labels []string, label string) bool {
	for _, l := range labels {
		if l == label {
			return true
		}
	}
	return false
}

// verifySignature checks that signature matches the HMAC-SHA256 of body using
// secret, in constant time to prevent timing attacks.
func verifySignature(body []byte, signature string, secret string) bool {

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}
