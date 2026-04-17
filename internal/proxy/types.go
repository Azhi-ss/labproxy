package proxy

import "time"

type Version struct {
	Version string `json:"version"`
	Meta    bool   `json:"meta"`
}

type Config struct {
	Mode               string `json:"mode"`
	MixedPort          int    `json:"mixed-port"`
	ExternalController string `json:"external-controller"`
}

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type DelayHistory struct {
	Time  string `json:"time"`
	Delay int    `json:"delay"`
}

type Proxy struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Now     string         `json:"now"`
	All     []string       `json:"all"`
	History []DelayHistory `json:"history"`
}

type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type ConnectionMetadata struct {
	Network     string `json:"network"`
	Type        string `json:"type"`
	SourceIP    string `json:"sourceIP"`
	Destination string `json:"destinationIP"`
	Host        string `json:"host"`
}

type Connection struct {
	ID       string             `json:"id"`
	Metadata ConnectionMetadata `json:"metadata"`
	Upload   int64              `json:"upload"`
	Download int64              `json:"download"`
	Start    time.Time          `json:"start"`
	Chains   []string           `json:"chains"`
	Rule     string             `json:"rule"`
}

type ConnectionsResponse struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}
