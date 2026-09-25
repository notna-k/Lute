package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegister(t *testing.T) {
	var got registerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/workers/bootstrap/register" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"worker_id":"abc123","grpc_address":"core:50051"}`))
	}))
	defer srv.Close()

	resp, err := register(srv.URL+"/", &registerRequest{Name: "build-box", ClaimCode: "CODE"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.WorkerID != "abc123" || resp.GRPCAddress != "core:50051" {
		t.Errorf("response = %+v", resp)
	}
	if got.Name != "build-box" || got.ClaimCode != "CODE" {
		t.Errorf("server received %+v", got)
	}
}

func TestRegisterErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"conflict", http.StatusConflict, `{"error":"host taken"}`, "already registered for this machine: host taken"},
		{"json error", http.StatusBadRequest, `{"error":"claim_code is required"}`, "server returned 400: claim_code is required"},
		{"plain error", http.StatusInternalServerError, "boom\n", "server returned 500: boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := register(srv.URL, &registerRequest{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestPromptServiceName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"  builder \n", "builder", false},
		{"no-newline", "no-newline", false},
		{"\n", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := promptServiceName(strings.NewReader(tt.in))
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("promptServiceName(%q) = %q, %v", tt.in, got, err)
		}
	}
}
