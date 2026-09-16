package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"tdtk/internal/config"
)

type profileItem struct{ p config.Profile }

func (i profileItem) Title() string { return i.p.Name }
func (i profileItem) Description() string {
	return fmt.Sprintf("%s  %s=%s  %s", i.p.Project, i.p.LocationType, i.p.Location, i.p.Cluster)
}
func (i profileItem) FilterValue() string { return i.p.Name }

type profileSelectModel struct {
	list      list.Model
	choice    string
	cancelled bool
}

// NewProfileSelect builds a picker over GKE cluster profiles.
func NewProfileSelect(profiles []config.Profile) tea.Model {
	items := make([]list.Item, len(profiles))
	for i, p := range profiles {
		items[i] = profileItem{p}
	}
	l := list.New(items, list.NewDefaultDelegate(), 60, 14)
	l.Title = "Select a cluster profile"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetFilteringEnabled(true)
	return profileSelectModel{list: l}
}

func (m profileSelectModel) Init() tea.Cmd { return nil }

func (m profileSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if sel, ok := m.list.SelectedItem().(profileItem); ok {
				m.choice = sel.p.Name
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m profileSelectModel) View() string { return m.list.View() }

// ProfileSelectResult extracts the picked profile name (and whether the
// user cancelled) from a finished tea.Model built with NewProfileSelect.
func ProfileSelectResult(m tea.Model) (choice string, cancelled bool) {
	s, _ := m.(profileSelectModel)
	return s.choice, s.cancelled
}
