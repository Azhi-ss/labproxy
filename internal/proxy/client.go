package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const DefaultDelayTestURL = "http://www.gstatic.com/generate_204"

type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	var out Version
	if err := c.getJSON(ctx, "/version", &out); err != nil {
		return Version{}, err
	}
	return out, nil
}

func (c *Client) Config(ctx context.Context) (Config, error) {
	var out Config
	if err := c.getJSON(ctx, "/configs", &out); err != nil {
		return Config{}, err
	}
	return out, nil
}

func (c *Client) Traffic(ctx context.Context) (Traffic, error) {
	var out Traffic
	if err := c.getJSON(ctx, "/traffic", &out); err != nil {
		return Traffic{}, err
	}
	return out, nil
}

func (c *Client) Proxies(ctx context.Context) (ProxiesResponse, error) {
	var out ProxiesResponse
	if err := c.getJSON(ctx, "/proxies", &out); err != nil {
		return ProxiesResponse{}, err
	}
	return out, nil
}

func (c *Client) Connections(ctx context.Context) (ConnectionsResponse, error) {
	var out ConnectionsResponse
	if err := c.getJSON(ctx, "/connections", &out); err != nil {
		return ConnectionsResponse{}, err
	}
	return out, nil
}

func (c *Client) Delay(ctx context.Context, proxyName string, timeout time.Duration) (int, error) {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return 0, fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/proxies", proxyName, "delay")
	query := endpoint.Query()
	query.Set("timeout", fmt.Sprintf("%d", timeout.Milliseconds()))
	query.Set("url", DefaultDelayTestURL)
	endpoint.RawQuery = query.Encode()

	req, err := c.newRequest(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("delay request failed: %s", strings.TrimSpace(string(body)))
	}

	var out struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode delay response: %w", err)
	}
	return out.Delay, nil
}

func (c *Client) SwitchProxy(ctx context.Context, groupName, proxyName string) error {
	payload, err := json.Marshal(map[string]string{"name": proxyName})
	if err != nil {
		return fmt.Errorf("marshal switch payload: %w", err)
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/proxies", groupName)

	req, err := c.newRequest(ctx, http.MethodPut, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("switch proxy failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s failed: %s", endpoint, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, target string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	return req, nil
}
