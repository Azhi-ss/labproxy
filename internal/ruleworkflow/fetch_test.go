package ruleworkflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCandidateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("payload: [DOMAIN-SUFFIX,github.com]\n"))
	}))
	defer server.Close()

	c := candidate("github", "GitHub", server.URL, "Proxies", "classical", "./rule-providers/github.yaml")
	got, err := FetchCandidate(context.Background(), server.Client(), c)
	if err != nil {
		t.Fatalf("FetchCandidate: %v", err)
	}
	if got.Candidate.Name != "github" {
		t.Fatalf("candidate name = %q", got.Candidate.Name)
	}
	if string(got.Data) == "" {
		t.Fatal("expected non-empty data")
	}
}

func TestFetchCandidateRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	c := candidate("github", "GitHub", server.URL, "Proxies", "classical", "./rule-providers/github.yaml")
	_, err := FetchCandidate(context.Background(), server.Client(), c)
	if err == nil {
		t.Fatal("expected http error")
	}
}
