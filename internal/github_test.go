package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v84/github"
)

func TestFetchJITConfig(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		wantConfig string
	}{
		{
			name:       "returns encoded JIT config",
			response:   `{"encoded_jit_config":"abc123base64"}`,
			statusCode: http.StatusCreated,
			wantConfig: "abc123base64",
		},
		{
			name:       "returns error when API returns 404",
			response:   `{"message":"Not Found"}`,
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "returns error when encoded_jit_config is absent",
			response:   `{}`,
			statusCode: http.StatusCreated,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				fmt.Fprintln(w, tt.response)
			}))
			defer srv.Close()

			gc := github.NewClient(nil)
			gc.BaseURL, _ = url.Parse(srv.URL + "/")

			client := &GitHubClient{client: gc}
			config, err := client.FetchJITConfig(context.Background(), "owner", "repo", "runner-1", 1, []string{"self-hosted"})

			if (err != nil) != tt.wantErr {
				t.Errorf("FetchJITConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && config != tt.wantConfig {
				t.Errorf("FetchJITConfig() = %v, want %v", config, tt.wantConfig)
			}
		})
	}
}
