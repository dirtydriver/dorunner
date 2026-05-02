package internal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			body:      []byte("test body"),
			secret:    "test-secret",
			signature: computeSignature([]byte("test body"), "test-secret"),
			want:      true,
		},
		{
			name:      "invalid signature",
			body:      []byte("test body"),
			secret:    "test-secret",
			signature: "sha256=invalidsignature",
			want:      false,
		},
		{
			name:      "wrong secret",
			body:      []byte("test body"),
			secret:    "test-secret",
			signature: computeSignature([]byte("test body"), "wrong-secret"),
			want:      false,
		},
		{
			name:      "empty body",
			body:      []byte(""),
			secret:    "test-secret",
			signature: computeSignature([]byte(""), "test-secret"),
			want:      true,
		},
		{
			name:      "signature without sha256 prefix",
			body:      []byte("test body"),
			secret:    "test-secret",
			signature: "invalidsignature",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifySignature(tt.body, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("verifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func computeSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           interface{}
		secret         string
		addSignature   bool
		validSignature bool
		wantStatus     int
	}{
		{
			name:           "rejects non-POST requests",
			method:         http.MethodGet,
			body:           nil,
			secret:         "test-secret",
			addSignature:   false,
			validSignature: false,
			wantStatus:     http.StatusMethodNotAllowed,
		},
		{
			name:           "rejects missing signature",
			method:         http.MethodPost,
			body:           WebhookPayload{Action: "queued"},
			secret:         "test-secret",
			addSignature:   false,
			validSignature: false,
			wantStatus:     http.StatusBadRequest,
		},
		{
			name:           "rejects invalid signature",
			method:         http.MethodPost,
			body:           WebhookPayload{Action: "queued"},
			secret:         "test-secret",
			addSignature:   true,
			validSignature: false,
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:   "accepts valid queued webhook",
			method: http.MethodPost,
			body: WebhookPayload{
				Action: "queued",
				Repository: Repository{
					FullName: "owner/repo",
				},
				WorkflowJob: WorkflowJob{
					ID:     12345,
					Labels: []string{"self-hosted", "linux"},
				},
			},
			secret:         "test-secret",
			addSignature:   true,
			validSignature: true,
			wantStatus:     http.StatusOK,
		},
		{
			name:   "accepts valid completed webhook",
			method: http.MethodPost,
			body: WebhookPayload{
				Action: "completed",
				Repository: Repository{
					FullName: "owner/repo",
				},
				WorkflowJob: WorkflowJob{
					RunnerName: "do-runner-12345",
				},
			},
			secret:         "test-secret",
			addSignature:   true,
			validSignature: true,
			wantStatus:     http.StatusOK,
		},
		{
			name:   "accepts valid webhook with unknown action",
			method: http.MethodPost,
			body: WebhookPayload{
				Action: "in_progress",
				Repository: Repository{
					FullName: "owner/repo",
				},
			},
			secret:         "test-secret",
			addSignature:   true,
			validSignature: true,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "rejects invalid JSON",
			method:         http.MethodPost,
			body:           "invalid json",
			secret:         "test-secret",
			addSignature:   true,
			validSignature: true,
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WebHookSecret:        tt.secret,
				GitHubRunnerGroupID:  1,
				GitHubRunnerLabels:   []string{"self-hosted", "linux"},
			}

			handler := NewWebhookHandler(cfg, nil, nil)

			var bodyBytes []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					bodyBytes = []byte(str)
				} else {
					bodyBytes, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/webhook", bytes.NewReader(bodyBytes))

			if tt.addSignature {
				if tt.validSignature {
					req.Header.Set("X-Hub-Signature-256", computeSignature(bodyBytes, tt.secret))
				} else {
					req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")
				}
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("ServeHTTP() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestNewWebhookHandler(t *testing.T) {
	t.Run("creates handler with all dependencies", func(t *testing.T) {
		cfg := &Config{
			WebHookSecret: "test-secret",
		}

		handler := NewWebhookHandler(cfg, nil, nil)

		if handler == nil {
			t.Error("NewWebhookHandler() returned nil")
		}
		if handler.cfg != cfg {
			t.Error("NewWebhookHandler() cfg not set correctly")
		}
	})
}

func TestWebhookPayload_Unmarshal(t *testing.T) {
	t.Run("unmarshals valid payload", func(t *testing.T) {
		jsonData := `{
			"action": "queued",
			"repository": {
				"full_name": "owner/repo"
			},
			"workflow_job": {
				"id": 12345,
				"labels": ["self-hosted", "linux"],
				"runner_name": "test-runner"
			}
		}`

		var payload WebhookPayload
		err := json.Unmarshal([]byte(jsonData), &payload)
		if err != nil {
			t.Errorf("Unmarshal() error = %v", err)
		}

		if payload.Action != "queued" {
			t.Errorf("Action = %v, want queued", payload.Action)
		}
		if payload.Repository.FullName != "owner/repo" {
			t.Errorf("Repository.FullName = %v, want owner/repo", payload.Repository.FullName)
		}
		if payload.WorkflowJob.ID != 12345 {
			t.Errorf("WorkflowJob.ID = %v, want 12345", payload.WorkflowJob.ID)
		}
		if len(payload.WorkflowJob.Labels) != 2 {
			t.Errorf("WorkflowJob.Labels length = %v, want 2", len(payload.WorkflowJob.Labels))
		}
		if payload.WorkflowJob.RunnerName != "test-runner" {
			t.Errorf("WorkflowJob.RunnerName = %v, want test-runner", payload.WorkflowJob.RunnerName)
		}
	})
}
