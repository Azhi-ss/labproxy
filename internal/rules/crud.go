package rules

import "fmt"

func (s *Store) AddRule(r Rule) (Diff, error) {
	if err := r.Validate(); err != nil {
		return Diff{}, fmt.Errorf("validate: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRules()
	if err != nil {
		return Diff{}, err
	}
	rules = append(rules, r)
	if err := s.saveRules(rules); err != nil {
		return Diff{}, err
	}
	return Diff{Added: []Rule{r}}, nil
}

func (s *Store) UpdateRule(index int, r Rule) (Diff, error) {
	if err := r.Validate(); err != nil {
		return Diff{}, fmt.Errorf("validate: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRules()
	if err != nil {
		return Diff{}, err
	}
	if index < 0 || index >= len(rules) {
		return Diff{}, fmt.Errorf("index %d out of range (0..%d)", index, len(rules)-1)
	}
	old := rules[index]
	rules[index] = r
	if err := s.saveRules(rules); err != nil {
		return Diff{}, err
	}
	return Diff{Modified: []Rule{r}, Removed: []Rule{old}}, nil
}

func (s *Store) DeleteRule(index int) (Diff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRules()
	if err != nil {
		return Diff{}, err
	}
	if index < 0 || index >= len(rules) {
		return Diff{}, fmt.Errorf("index %d out of range (0..%d)", index, len(rules)-1)
	}
	removed := rules[index]
	rules = append(rules[:index], rules[index+1:]...)
	if err := s.saveRules(rules); err != nil {
		return Diff{}, err
	}
	return Diff{Removed: []Rule{removed}}, nil
}
