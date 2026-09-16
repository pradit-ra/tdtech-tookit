package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type stringItem string

func (i stringItem) Title() string       { return string(i) }
func (i stringItem) Description() string { return "" }
func (i stringItem) FilterValue() string { return string(i) }

type selectModel struct {
	list      list.Model
	choice    string
	cancelled bool
}

// NewStringSelect builds a single-column list picker over plain strings.
func NewStringSelect(title string, options []string) tea.Model {
	items := make([]list.Item, len(options))
	for i, o := range options {
		items[i] = stringItem(o)
	}
	l := list.New(items, list.NewDefaultDelegate(), 60, 14)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	return selectModel{list: l}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if sel, ok := m.list.SelectedItem().(stringItem); ok {
				m.choice = string(sel)
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() string { return m.list.View() }

// StringSelectResult extracts the picked option (and whether the user
// cancelled) from a finished tea.Model built with NewStringSelect.
func StringSelectResult(m tea.Model) (choice string, cancelled bool) {
	s, _ := m.(selectModel)
	return s.choice, s.cancelled
}
