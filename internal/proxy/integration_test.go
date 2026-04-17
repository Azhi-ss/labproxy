package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntegration_Version(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Version{
			Version: "mihomo-test-v1.0.0",
			Meta:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}

	if version.Version != "mihomo-test-v1.0.0" {
		t.Errorf("Version.Version = %q, want %q", version.Version, "mihomo-test-v1.0.0")
	}
	if !version.Meta {
		t.Error("Version.Meta = false, want true")
	}
}

func TestIntegration_Config(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Config{
			Mode:               "rule",
			MixedPort:          7894,
			ExternalController: "127.0.0.1:9090",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	config, err := client.Config(context.Background())
	if err != nil {
		t.Fatalf("Config() error = %v", err)
	}

	if config.Mode != "rule" {
		t.Errorf("Config.Mode = %q, want %q", config.Mode, "rule")
	}
	if config.MixedPort != 7894 {
		t.Errorf("Config.MixedPort = %d, want %d", config.MixedPort, 7894)
	}
}

func TestIntegration_Proxies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProxiesResponse{
			Proxies: map[string]Proxy{
				"GLOBAL": {
					Name: "GLOBAL",
					Type: "Selector",
					Now:  "DIRECT",
					All:  []string{"Proxy", "DIRECT", "REJECT"},
				},
				"DIRECT": {
					Name: "DIRECT",
					Type: "Direct",
				},
				"REJECT": {
					Name: "REJECT",
					Type: "Reject",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	proxies, err := client.Proxies(context.Background())
	if err != nil {
		t.Fatalf("Proxies() error = %v", err)
	}

	if len(proxies.Proxies) != 3 {
		t.Errorf("len(Proxies) = %d, want 3", len(proxies.Proxies))
	}

	global, ok := proxies.Proxies["GLOBAL"]
	if !ok {
		t.Fatal("GLOBAL proxy not found")
	}
	if global.Type != "Selector" {
		t.Errorf("GLOBAL.Type = %q, want %q", global.Type, "Selector")
	}
	if global.Now != "DIRECT" {
		t.Errorf("GLOBAL.Now = %q, want %q", global.Now, "DIRECT")
	}
}

func TestIntegration_Traffic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traffic" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Traffic{
			Up:   1024,
			Down: 2048,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	traffic, err := client.Traffic(context.Background())
	if err != nil {
		t.Fatalf("Traffic() error = %v", err)
	}

	if traffic.Up != 1024 {
		t.Errorf("Traffic.Up = %d, want %d", traffic.Up, 1024)
	}
	if traffic.Down != 2048 {
		t.Errorf("Traffic.Down = %d, want %d", traffic.Down, 2048)
	}
}

func TestIntegration_Delay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies/Test-Proxy/delay" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("timeout") != "1000" {
			http.Error(w, "invalid timeout", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("url") != "http://www.gstatic.com/generate_204" {
			http.Error(w, "invalid url", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"delay": 123,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	delay, err := client.Delay(context.Background(), "Test-Proxy", 1000*time.Millisecond)
	if err != nil {
		t.Fatalf("Delay() error = %v", err)
	}

	if delay != 123 {
		t.Errorf("Delay() = %d, want %d", delay, 123)
	}
}

func TestIntegration_SwitchProxy(t *testing.T) {
	var receivedGroup, receivedProxy string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/proxies/GLOBAL" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		receivedGroup = "GLOBAL"
		receivedProxy = req.Name

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.SwitchProxy(context.Background(), "GLOBAL", "Test-Proxy")
	if err != nil {
		t.Fatalf("SwitchProxy() error = %v", err)
	}

	if receivedGroup != "GLOBAL" {
		t.Errorf("received group = %q, want %q", receivedGroup, "GLOBAL")
	}
	if receivedProxy != "Test-Proxy" {
		t.Errorf("received proxy = %q, want %q", receivedProxy, "Test-Proxy")
	}
}

func TestIntegration_WithSecret(t *testing.T) {
	const testSecret = "my-test-secret-12345"
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Version{
			Version: "test",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, testSecret)
	_, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}

	if authHeader != "Bearer "+testSecret {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer "+testSecret)
	}
}

func TestIntegration_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Version{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Version(ctx)
	if err == nil {
		t.Fatal("Version() expected error, got nil")
	}
	if ctx.Err() == nil {
		t.Fatal("context should have timed out")
	}
}

func TestIntegration_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() expected error, got nil")
	}
}

func TestIntegration_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() expected error, got nil")
	}
}
