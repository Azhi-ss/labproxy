package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
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

// DelayGroup 并发测试一个代理组内所有节点的延迟。
// 返回 map[name]int；失败或超时的节点记为 -1，不阻断整体。
// 并发安全（用 sync.Mutex 保护 map 写入）。
func (c *Client) DelayGroup(ctx context.Context, group Proxy, timeout time.Duration) (map[string]int, error) {
	result := make(map[string]int)
	if len(group.All) == 0 {
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, name := range group.All {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			delay, err := c.Delay(ctx, n, timeout)
			val := delay
			if err != nil {
				val = -1
			}
			mu.Lock()
			result[n] = val
			mu.Unlock()
		}(name)
	}
	wg.Wait()
	return result, nil
}

// Logs 订阅 mihomo /logs 流，逐行产出 LogEntry。
// level 控制日志级别（debug/info/warning/error/silent），空则用 info。
// ctx 取消会关闭连接与 channel，调用方应从 channel 读完直到其关闭。
// 解析失败的行被跳过，不阻断流。
func (c *Client) Logs(ctx context.Context, level string) <-chan LogEntry {
	out := make(chan LogEntry)
	if level == "" {
		level = "info"
	}

	go func() {
		defer close(out)

		endpoint, err := url.Parse(c.baseURL)
		if err != nil {
			return
		}
		endpoint.Path = path.Join(endpoint.Path, "/logs")
		q := endpoint.Query()
		q.Set("level", level)
		endpoint.RawQuery = q.Encode()

		req, err := c.newRequest(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// mihomo 日志单行通常较短，但放大缓冲以容错
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var entry LogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue // 跳过无法解析的行
			}
			select {
			case out <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

// DNSQuery 调用 mihomo /dns/query 解析域名。
// name 为空时直接报错；qtype 为 DNS 记录类型（A/AAAA/CNAME/MX/TXT 等），空默认 A。
// 不支持 /dns/query 的内核会返回错误，调用方应优雅降级。
func (c *Client) DNSQuery(ctx context.Context, name, qtype string) (DNSQueryResponse, error) {
	if strings.TrimSpace(name) == "" {
		return DNSQueryResponse{}, fmt.Errorf("dns query failed: empty name")
	}
	if qtype == "" {
		qtype = "A"
	}

	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return DNSQueryResponse{}, fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/dns/query")
	q := endpoint.Query()
	q.Set("name", name)
	q.Set("type", qtype)
	endpoint.RawQuery = q.Encode()

	req, err := c.newRequest(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return DNSQueryResponse{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DNSQueryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return DNSQueryResponse{}, fmt.Errorf("dns query failed: %s", strings.TrimSpace(string(body)))
	}

	var out DNSQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return DNSQueryResponse{}, fmt.Errorf("decode dns response: %w", err)
	}
	return out, nil
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

func (c *Client) UpdateMode(ctx context.Context, mode string) error {
	payload, err := json.Marshal(map[string]string{"mode": mode})
	if err != nil {
		return fmt.Errorf("marshal mode payload: %w", err)
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/configs")

	req, err := c.newRequest(ctx, http.MethodPatch, endpoint.String(), bytes.NewReader(payload))
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
		return fmt.Errorf("update mode failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

// CloseConnection 关闭指定连接。对应 mihomo DELETE /connections/:id。
// id 为空时直接返回错误，避免误删全部连接。
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("close connection failed: empty connection id")
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/connections", id)

	req, err := c.newRequest(ctx, http.MethodDelete, endpoint.String(), nil)
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
		return fmt.Errorf("close connection failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

// CloseAllConnections 关闭所有连接。对应 mihomo DELETE /connections。
func (c *Client) CloseAllConnections(ctx context.Context) error {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/connections")

	req, err := c.newRequest(ctx, http.MethodDelete, endpoint.String(), nil)
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
		return fmt.Errorf("close all connections failed: %s", strings.TrimSpace(string(body)))
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
