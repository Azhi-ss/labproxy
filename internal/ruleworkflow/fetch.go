package ruleworkflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const MaxRuleSourceBytes = 5 * 1024 * 1024

type FetchedSource struct {
	Candidate Candidate
	Data      []byte
}

func FetchCandidate(ctx context.Context, client *http.Client, c Candidate) (FetchedSource, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.SourceURL, nil)
	if err != nil {
		return FetchedSource{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return FetchedSource{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return FetchedSource{}, fmt.Errorf("fetch %s: http %d", c.Name, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxRuleSourceBytes+1))
	if err != nil {
		return FetchedSource{}, err
	}
	if len(data) > MaxRuleSourceBytes {
		return FetchedSource{}, fmt.Errorf("fetch %s: source exceeds %d bytes", c.Name, MaxRuleSourceBytes)
	}

	return FetchedSource{
		Candidate: c,
		Data:      data,
	}, nil
}
