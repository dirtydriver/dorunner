package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/digitalocean/godo"
)

func TestRenderUserData(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		data    BootstrapData
		wantErr bool
		want    string
	}{
		{
			name:   "renders simple template",
			script: "JIT={{.JITConfig}} VERSION={{.RunnerVersion}}",
			data: BootstrapData{
				JITConfig:     "test-jit-config",
				RunnerVersion: "2.334.0",
			},
			wantErr: false,
			want:    "JIT=test-jit-config VERSION=2.334.0",
		},
		{
			name:   "renders empty template",
			script: "",
			data: BootstrapData{
				JITConfig:     "test",
				RunnerVersion: "1.0.0",
			},
			wantErr: false,
			want:    "",
		},
		{
			name:   "renders multiline template",
			script: "#!/bin/bash\nJIT={{.JITConfig}}\nVERSION={{.RunnerVersion}}",
			data: BootstrapData{
				JITConfig:     "abc123",
				RunnerVersion: "2.400.0",
			},
			wantErr: false,
			want:    "#!/bin/bash\nJIT=abc123\nVERSION=2.400.0",
		},
		{
			name:   "fails on invalid template syntax",
			script: "{{.Invalid",
			data: BootstrapData{
				JITConfig:     "test",
				RunnerVersion: "1.0.0",
			},
			wantErr: true,
		},
		{
			name:   "fails on undefined field",
			script: "{{.NonExistentField}}",
			data: BootstrapData{
				JITConfig:     "test",
				RunnerVersion: "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderUserData(tt.script, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderUserData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("renderUserData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name      string
		labels    []string
		snapshots []godo.Snapshot
		want      *DropletConfig
		wantErr   bool
	}{
		{
			name:   "parses region, size, and image labels",
			labels: []string{"region/nyc1", "size/s-2vcpu-4gb", "image/ubuntu-22-04-x64"},
			want: &DropletConfig{
				Region:        "nyc1",
				Size:          "s-2vcpu-4gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "uses defaults when no labels provided",
			labels: []string{},
			want: &DropletConfig{
				Region:        "fra1",
				Size:          "s-1vcpu-2gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "parses snapshot label",
			labels: []string{"region/lon1", "size/s-4vcpu-8gb", "snapshot/my-runner-image"},
			snapshots: []godo.Snapshot{
				{ID: "12345", Name: "my-runner-image"},
			},
			want: &DropletConfig{
				Region:        "lon1",
				Size:          "s-4vcpu-8gb",
				Image:         godo.DropletCreateImage{ID: 12345},
				BootStrapType: "packer",
			},
			wantErr: false,
		},
		{
			name:   "ignores non-prefixed labels",
			labels: []string{"self-hosted", "linux", "x64", "region/ams3"},
			want: &DropletConfig{
				Region:        "ams3",
				Size:          "s-1vcpu-2gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "fails when snapshot not found",
			labels: []string{"snapshot/nonexistent"},
			snapshots: []godo.Snapshot{
				{ID: "12345", Name: "different-snapshot"},
			},
			wantErr: true,
		},
		{
			name:   "uses default region when not specified",
			labels: []string{"size/s-2vcpu-4gb", "image/ubuntu-20-04-x64"},
			want: &DropletConfig{
				Region:        "fra1",
				Size:          "s-2vcpu-4gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-20-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "uses default size when not specified",
			labels: []string{"region/sfo3", "image/ubuntu-22-04-x64"},
			want: &DropletConfig{
				Region:        "sfo3",
				Size:          "s-1vcpu-2gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "uses default image when not specified",
			labels: []string{"region/tor1", "size/s-2vcpu-2gb"},
			want: &DropletConfig{
				Region:        "tor1",
				Size:          "s-2vcpu-2gb",
				Image:         godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
				BootStrapType: "stock",
			},
			wantErr: false,
		},
		{
			name:   "handles multiple snapshots and finds correct one",
			labels: []string{"snapshot/target-snapshot"},
			snapshots: []godo.Snapshot{
				{ID: "11111", Name: "snapshot-1"},
				{ID: "22222", Name: "target-snapshot"},
				{ID: "33333", Name: "snapshot-3"},
			},
			want: &DropletConfig{
				Region:        "fra1",
				Size:          "s-1vcpu-2gb",
				Image:         godo.DropletCreateImage{ID: 22222},
				BootStrapType: "packer",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &DOClient{
				snapshots:     tt.snapshots,
				runnerVersion: "2.334.0",
			}

			got, err := client.parseLabels(tt.labels)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Region != tt.want.Region {
				t.Errorf("parseLabels() Region = %v, want %v", got.Region, tt.want.Region)
			}
			if got.Size != tt.want.Size {
				t.Errorf("parseLabels() Size = %v, want %v", got.Size, tt.want.Size)
			}
			if got.Image.Slug != tt.want.Image.Slug {
				t.Errorf("parseLabels() Image.Slug = %v, want %v", got.Image.Slug, tt.want.Image.Slug)
			}
			if got.Image.ID != tt.want.Image.ID {
				t.Errorf("parseLabels() Image.ID = %v, want %v", got.Image.ID, tt.want.Image.ID)
			}
			if got.BootStrapType != tt.want.BootStrapType {
				t.Errorf("parseLabels() BootStrapType = %v, want %v", got.BootStrapType, tt.want.BootStrapType)
			}
		})
	}
}

func newTestDOClient(t *testing.T, handler http.HandlerFunc) (*DOClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	gc := godo.NewFromToken("fake-token")
	gc.BaseURL, _ = url.Parse(srv.URL + "/")
	return &DOClient{client: gc}, srv
}

func TestGetDropletId(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantID     int
		wantErr    bool
	}{
		{
			name:       "returns ID when droplet exists",
			response:   `{"droplets": [{"id": 42, "name": "runner-123"}]}`,
			statusCode: http.StatusOK,
			wantID:     42,
		},
		{
			name:       "returns 0 nil when droplet not found",
			response:   `{"droplets": []}`,
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "returns error on API failure",
			response:   `{"id": "server_error", "message": "internal"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, srv := newTestDOClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				fmt.Fprintln(w, tt.response)
			})
			defer srv.Close()

			id, err := client.getDropletId(context.Background(), "runner-123")
			if (err != nil) != tt.wantErr {
				t.Errorf("getDropletId() error = %v, wantErr %v", err, tt.wantErr)
			}
			if id != tt.wantID {
				t.Errorf("getDropletId() = %d, want %d", id, tt.wantID)
			}
		})
	}
}

func TestDeleteDroplet(t *testing.T) {
	t.Run("skips without error when droplet already gone", func(t *testing.T) {
		client, srv := newTestDOClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"droplets": []}`)
		})
		defer srv.Close()

		if err := client.DeleteDroplet(context.Background(), "runner-gone"); err != nil {
			t.Errorf("DeleteDroplet() unexpected error for already-gone droplet: %v", err)
		}
	})

	t.Run("deletes droplet when found", func(t *testing.T) {
		deleted := false
		client, srv := newTestDOClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodDelete {
				deleted = true
				w.WriteHeader(http.StatusNoContent)
				return
			}
			fmt.Fprintln(w, `{"droplets": [{"id": 42, "name": "runner-123"}]}`)
		})
		defer srv.Close()

		if err := client.DeleteDroplet(context.Background(), "runner-123"); err != nil {
			t.Errorf("DeleteDroplet() unexpected error: %v", err)
		}
		if !deleted {
			t.Error("DeleteDroplet() did not call the delete API")
		}
	})

	t.Run("returns error on list API failure", func(t *testing.T) {
		client, srv := newTestDOClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{"id":"server_error","message":"internal"}`)
		})
		defer srv.Close()

		if err := client.DeleteDroplet(context.Background(), "runner-123"); err == nil {
			t.Error("DeleteDroplet() expected error on API failure, got nil")
		}
	})
}

func TestStartCleanupCron(t *testing.T) {
	t.Run("stops when context is cancelled", func(t *testing.T) {
		client := &DOClient{
			runnerVersion: "2.334.0",
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool)

		go func() {
			client.StartCleanupCron(ctx, 100*time.Millisecond, time.Hour)
			done <- true
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("StartCleanupCron did not stop after context cancellation")
		}
	})
}

func TestStartSnapshotRefresher(t *testing.T) {
	t.Run("stops when context is cancelled", func(t *testing.T) {
		client := &DOClient{
			runnerVersion: "2.334.0",
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool)

		go func() {
			client.StartSnapshotRefresher(ctx, 100*time.Millisecond)
			done <- true
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("StartSnapshotRefresher did not stop after context cancellation")
		}
	})
}
