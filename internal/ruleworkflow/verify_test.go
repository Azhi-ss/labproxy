package ruleworkflow

import (
	"context"
	"testing"

	"labproxy/internal/proxy"
)

type fakeRuntimeClient struct{}

func (fakeRuntimeClient) Proxies(context.Context) (proxy.ProxiesResponse, error) {
	return proxy.ProxiesResponse{Proxies: map[string]proxy.Proxy{
		"OpenAI":  {Name: "OpenAI", Type: "Selector"},
		"Proxies": {Name: "Proxies", Type: "Selector"},
	}}, nil
}

func (fakeRuntimeClient) Connections(context.Context) (proxy.ConnectionsResponse, error) {
	return proxy.ConnectionsResponse{Connections: []proxy.Connection{{ID: "1", Chains: []string{"JP", "OpenAI"}}}}, nil
}

func TestInspectRuntime(t *testing.T) {
	got, err := InspectRuntime(context.Background(), fakeRuntimeClient{})
	if err != nil {
		t.Fatalf("InspectRuntime: %v", err)
	}
	if !got.StrategyGroups["OpenAI"] || got.ConnectionCount != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

type allGroupTypesRuntimeClient struct{}

func (allGroupTypesRuntimeClient) Proxies(context.Context) (proxy.ProxiesResponse, error) {
	return proxy.ProxiesResponse{Proxies: map[string]proxy.Proxy{
		"selector":     {Name: "selector", Type: "Selector"},
		"url-test":     {Name: "url-test", Type: "URLTest"},
		"fallback":     {Name: "fallback", Type: "Fallback"},
		"load-balance": {Name: "load-balance", Type: "LoadBalance"},
		"plain-node":   {Name: "plain-node", Type: "SS"},
	}}, nil
}

func (allGroupTypesRuntimeClient) Connections(context.Context) (proxy.ConnectionsResponse, error) {
	return proxy.ConnectionsResponse{Connections: []proxy.Connection{{ID: "1"}, {ID: "2"}}}, nil
}

func TestInspectRuntimeClassifiesStrategyGroupTypes(t *testing.T) {
	got, err := InspectRuntime(context.Background(), allGroupTypesRuntimeClient{})
	if err != nil {
		t.Fatalf("InspectRuntime: %v", err)
	}
	for _, name := range []string{"selector", "url-test", "fallback", "load-balance"} {
		if !got.StrategyGroups[name] {
			t.Fatalf("missing strategy group %q in %+v", name, got)
		}
	}
	if got.StrategyGroups["plain-node"] {
		t.Fatalf("plain node classified as strategy group: %+v", got)
	}
	if got.ConnectionCount != 2 {
		t.Fatalf("ConnectionCount = %d, want 2", got.ConnectionCount)
	}
}
