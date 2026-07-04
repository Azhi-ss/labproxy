package rules

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (s *Store) AddProvider(p Provider) (Diff, error) {
	if err := ValidateProvider(p); err != nil {
		return Diff{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	providers, err := s.loadProviders()
	if err != nil {
		return Diff{}, err
	}
	for _, existing := range providers {
		if existing.Name == p.Name {
			return Diff{}, fmt.Errorf("provider %q already exists", p.Name)
		}
	}
	providers = append(providers, p)
	if err := s.saveProviders(providers); err != nil {
		return Diff{}, err
	}
	return Diff{}, nil
}

func (s *Store) DeleteProvider(name string) (Diff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers, err := s.loadProviders()
	if err != nil {
		return Diff{}, err
	}
	out := make([]Provider, 0, len(providers))
	found := false
	for _, p := range providers {
		if p.Name == name {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return Diff{}, fmt.Errorf("provider %q not found", name)
	}
	if err := s.saveProviders(out); err != nil {
		return Diff{}, err
	}
	return Diff{}, nil
}

func (s *Store) RefreshProvider(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers, err := s.loadProviders()
	if err != nil {
		return err
	}
	var p *Provider
	for i := range providers {
		if providers[i].Name == name {
			p = &providers[i]
			break
		}
	}
	if p == nil {
		return fmt.Errorf("provider %q not found", name)
	}
	if p.Type != "http" {
		return fmt.Errorf("only http providers can be refreshed")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(p.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	target := p.Path
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(s.Path), target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, 5*1024*1024))
	return err
}
