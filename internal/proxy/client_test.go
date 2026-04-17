package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		secret   string
		expected string
	}{
		{"with secret", "http://localhost:9090", "my-secret", "http://localhost:9090"},
		{"without secret", "http://localhost:9090", "", "http://localhost:9090"},
		{"with trailing slash", "http://localhost:9090/", "secret", "http://localhost:9090"},
		{"with multiple trailing slashes", "http://localhost:9090///", "secret", "http://localhost:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.secret)
			if client.baseURL != tt.expected {
				t.Fatalf("expected base URL %q, got %q", tt.expected, client.baseURL)
			}
			if client.secret != tt.secret {
				t.Fatalf("expected secret %q, got %q", tt.secret, client.secret)
			}
			if client.httpClient == nil {
				t.Fatal("expected http client to be initialized")
			}
		})
	}
}

func TestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(Version{
			Version: "mihomo-meta v1.18.0",
			Meta:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if version.Version != "mihomo-meta v1.18.0" {
		t.Fatalf("expected version 'mihomo-meta v1.18.0', got %q", version.Version)
	}
	if !version.Meta {
		t.Fatal("expected meta to be true")
	}
}

func TestVersion_WithSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-secret" {
			t.Fatalf("expected authorization header 'Bearer test-secret', got %q", authHeader)
		}
		json.NewEncoder(w).Encode(Version{
			Version: "mihomo v1.18.0",
			Meta:    false,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-secret")
	ctx := context.Background()

	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if version.Version != "mihomo v1.18.0" {
		t.Fatalf("expected version 'mihomo v1.18.0', got %q", version.Version)
	}
}

func TestConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Config{
			Mode:               "rule",
			MixedPort:          7890,
			ExternalController: "1270.0.0.1:9090",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	config, err := client.Config(ctx)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if config.Mode != "rule" {
		t.Fatalf("expected mode 'rule', got %q", config.Mode)
	}
	if config.MixedPort != 7890 {
		t.Fatalf("expected mixed port 7890, got %d", config.MixedPort)
	}
}

func TestTraffic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traffic" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Traffic{
			Up:   1024 * 1024 * 10, // 10MB
			Down: 1024 * 1024 * 50, // 50MB
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	traffic, err := client.Traffic(ctx)
	if err != nil {
		t.Fatalf("Traffic() error: %v", err)
	}
	if traffic.Up != 1024*1024*10 {
		t.Fatalf("expected up traffic %d, got %d", 1024*1024*10, traffic.Up)
	}
	if traffic.Down != 1024*1024*50 {
		t.Fatalf("expected down traffic %d, got %d", 1024*1024*50, traffic.Down)
	}
}

func TestProxies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ProxiesResponse{
			Proxies: map[string]Proxy{
				"GLOBAL": {
					Name: "GLOBAL",
					Type: "Selector",
					Now:  "Node-A",
					All:  []string{"Node-A", "Node-B"},
					History: []DelayHistory{
						{Time: "2024-01-01T00:00:00Z", Delay: 42},
					},
				},
				"Node-A": {
					Name: "Node-A",
					Type: "SS",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	proxies, err := client.Proxies(ctx)
	if err != nil {
		t.Fatalf("Proxies() error: %v", err)
	}
	if len(proxies.Proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies.Proxies))
	}
	if proxies.Proxies["GLOBAL"].Type != "Selector" {
		t.Fatalf("expected GLOBAL type 'Selector', got %q", proxies.Proxies["GLOBAL"].Type)
	}
}

func TestConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ConnectionsResponse{
			DownloadTotal: 1024 * 1024 * 100,
			UploadTotal:   1024 * 1024 * 20,
			Connections: []Connection{
				{
					ID: "conn-1",
					Metadata: ConnectionMetadata{
						Network:     "TCP",
						Type:        "HTTP",
						SourceIP:    "192.168.1.1",
						Destination: "8.8.8.8:443",
						Host:        "example.com",
					},
					Upload:   1024,
					Download: 2048,
					Chains:   []string{"GLOBAL", "Node-A"},
					Rule:     "MATCH",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	connections, err := client.Connections(ctx)
	if err != nil {
		t.Fatalf("Connections() error: %v", err)
	}
	if len(connections.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(connections.Connections))
	}
	if connections.Connections[0].ID != "conn-1" {
		t.Fatalf("expected connection ID 'conn-1', got %q", connections.Connections[0].ID)
	}
}

func TestDelay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/proxies/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasSuffix(r.URL.Path, "/delay") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		timeout := r.URL.Query().Get("timeout")
		testURL := r.URL.Query().Get("url")

		if timeout != "5000" {
			t.Fatalf("expected timeout '5000', got %q", timeout)
		}
		if testURL != DefaultDelayTestURL {
			t.Fatalf("expected test URL %q, got %q", DefaultDelayTestURL, testURL)
		}

		json.NewEncoder(w).Encode(map[string]int{"delay": 42})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	delay, err := client.Delay(ctx, "Node-A", 5*time.Second)
	if err != nil {
		t.Fatalf("Delay() error: %v", err)
	}
	if delay != 42 {
		t.Fatalf("expected delay 42ms, got %d", delay)
	}
}

func TestDelay_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("delay test failed"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	_, err := client.Delay(ctx, "Node-A", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for failed delay request")
	}
	if !strings.Contains(err.Error(), "delay request failed") {
		t.Fatalf("expected error message to contain 'delay request failed', got %q", err.Error())
	}
}

func TestSwitchProxy(t *testing.T) {
	var switchedGroupName, switchedProxyName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/proxies/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if strings.HasSuffix(r.URL.Path, "/delay") {
			// This is a delay request
			json.NewEncoder(w).Encode(map[string]int{"delay": 42})
			return
		}

		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT method, got %s", r.Method)
		}

		// Extract group name from path
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 3 {
			t.Fatalf("unexpected path format: %s", r.URL.Path)
		}
		switchedGroupName = pathParts[2]

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		switchedProxyName = payload["name"]

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	err := client.SwitchProxy(ctx, "GLOBAL", "Node-B")
	if err != nil {
		t.Fatalf("SwitchProxy() error: %v", err)
	}
	if switchedGroupName != "GLOBAL" {
		t.Fatalf("expected group name 'GLOBAL', got %q", switchedGroupName)
	}
	if switchedProxyName != "Node-B" {
		t.Fatalf("expected proxy name 'Node-B', got %q", switchedProxyName)
	}
}

func TestSwitchProxy_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("proxy not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	err := client.SwitchProxy(ctx, "GLOBAL", "Node-A")
	if err == nil {
		t.Fatal("expected error for failed switch request")
	}
	if !strings.Contains(err.Error(), "switch proxy failed") {
		t.Fatalf("expected error message to contain 'switch proxy failed', got %q", err.Error())
	}
}

func TestSwitchProxy_WithSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-secret" {
			t.Fatalf("expected authorization header 'Bearer test-secret', got %q", authHeader)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-secret")
	ctx := context.Background()

	err := client.SwitchProxy(ctx, "GLOBAL", "Node-A")
	if err != nil {
		t.Fatalf("SwitchProxy() error: %v", err)
	}
}

func TestUpdateMode(t *testing.T) {
	var receivedMode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH method, got %s", r.Method)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		receivedMode = payload["mode"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	if err := client.UpdateMode(ctx, "global"); err != nil {
		t.Fatalf("UpdateMode() error: %v", err)
	}
	if receivedMode != "global" {
		t.Fatalf("expected mode 'global', got %q", receivedMode)
	}
}

func TestUpdateMode_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("mode not allowed"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	err := client.UpdateMode(ctx, "global")
	if err == nil {
		t.Fatal("expected error for failed mode update")
	}
	if !strings.Contains(err.Error(), "update mode failed") {
		t.Fatalf("expected error message to contain 'update mode failed', got %q", err.Error())
	}
}

func TestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	tests := []struct {
		name    string
		runTest func() error
	}{
		{"Version error", func() error {
			_, err := client.Version(ctx)
			return err
		}},
		{"Config error", func() error {
			_, err := client.Config(ctx)
			return err
		}},
		{"Traffic error", func() error {
			_, err := client.Traffic(ctx)
			return err
		}},
		{"Proxies error", func() error {
			_, err := client.Proxies(ctx)
			return err
		}},
		{"Connections error", func() error {
			_, err := client.Connections(ctx)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.runTest()
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx := context.Background()

	_, err := client.Version(ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNetworkError(t *testing.T) {
	// Use a URL that will definitely fail
	client := NewClient("http://localhost:9999", "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Version(ctx)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}
