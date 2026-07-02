package ruleworkflow

import (
	"context"

	"labproxy/internal/proxy"
)

type RuntimeClient interface {
	Proxies(context.Context) (proxy.ProxiesResponse, error)
	Connections(context.Context) (proxy.ConnectionsResponse, error)
}

type RuntimeSummary struct {
	StrategyGroups  map[string]bool
	ConnectionCount int
}

func InspectRuntime(ctx context.Context, client RuntimeClient) (RuntimeSummary, error) {
	proxies, err := client.Proxies(ctx)
	if err != nil {
		return RuntimeSummary{}, err
	}
	conns, err := client.Connections(ctx)
	if err != nil {
		return RuntimeSummary{}, err
	}
	groups := map[string]bool{}
	for name, p := range proxies.Proxies {
		switch p.Type {
		case "Selector", "URLTest", "Fallback", "LoadBalance":
			groups[name] = true
		}
	}
	return RuntimeSummary{StrategyGroups: groups, ConnectionCount: len(conns.Connections)}, nil
}
