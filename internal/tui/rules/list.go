package rules

import (
	"fmt"

	"labproxy/internal/rules"
)

// formatRow renders a single rule as a list row.
//   - marker: ● when enabled, ○ when disabled
//   - Type left-padded to 15
//   - Payload truncated to 30 chars (with "..." suffix)
//   - Proxy destination after an arrow
func formatRow(idx int, r rules.Rule, width int) string {
	marker := "○"
	if r.Enabled {
		marker = "●"
	}
	payload := r.Payload
	if len(payload) > 30 {
		payload = payload[:27] + "..."
	}
	return fmt.Sprintf(" %s %-15s %-30s → %s", marker, r.Type, payload, r.Proxy)
}
