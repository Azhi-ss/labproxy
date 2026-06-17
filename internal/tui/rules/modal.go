package rules

import (
	"sync"

	"labproxy/internal/rules"

	tea "github.com/charmbracelet/bubbletea"
)

// View identifies a screen inside the rules modal.
type View int

const (
	ViewMenu View = iota
	ViewList
	ViewForm
	ViewProviders
	ViewImport
)

// Modal is the rules-manager overlay. It is safe for concurrent use.
type Modal struct {
	path   string
	store  *rules.Store
	mu     sync.Mutex
	open   bool
	view   View
	cursor int
}

// NewModal builds a modal bound to the given mixin config path.
// Store creation is best-effort: an empty path yields a nil store,
// which the modal tolerates by no-oping store operations.
func NewModal(mixinPath string) *Modal {
	store, _ := rules.NewStore(mixinPath)
	return &Modal{
		path:  mixinPath,
		store: store,
		view:  ViewMenu,
	}
}

// Open shows the modal at its menu view.
func (m *Modal) Open() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = true
	m.view = ViewMenu
	m.cursor = 0
}

// Close hides the modal.
func (m *Modal) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = false
}

// IsOpen reports whether the modal is currently displayed.
func (m *Modal) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.open
}

// View renders the current screen. Returns "" when closed.
func (m *Modal) View() string {
	if !m.IsOpen() {
		return ""
	}
	return "[Rules Modal — view: " + viewName(m.view) + "]"
}

func viewName(v View) string {
	switch v {
	case ViewList:
		return "list"
	case ViewForm:
		return "form"
	case ViewProviders:
		return "providers"
	case ViewImport:
		return "import"
	default:
		return "menu"
	}
}

// Update handles a key message. Returns true if the key was consumed.
//   - esc / R: from Menu → close; from a sub-view → back to Menu
//   - 1: rule list
//   - 2: rule providers
//   - 3: import rules
//   - 4: reset rules to default (best-effort)
//
// Any other key is consumed (returns true) so the modal keeps focus.
func (m *Modal) Update(msg tea.KeyMsg) bool {
	if !m.IsOpen() {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch msg.String() {
	case "esc", "R":
		if m.view == ViewMenu {
			m.open = false
		} else {
			m.view = ViewMenu
		}
		return true
	case "1":
		m.view = ViewList
		return true
	case "2":
		m.view = ViewProviders
		return true
	case "3":
		m.view = ViewImport
		return true
	case "4":
		if m.store != nil {
			_, _ = m.store.ResetRules()
		}
		return true
	}
	return true
}
