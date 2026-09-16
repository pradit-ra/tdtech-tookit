// Package tui holds the bubbletea models for tdtk's interactive flows.
package tui

import (
	"fmt"
	"regexp"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

var portMapRe = regexp.MustCompile(`^[0-9]+:[0-9]+$`)

type portMapModel struct {
	port      textinput.Model
	err       string
	value     string
	cancelled bool
}

// NewPortMapInput prompts for a "local:remote" port mapping, e.g. 8080:80.
func NewPortMapInput() tea.Model {
	port := textinput.New()
	port.Placeholder = "local:remote, e.g. 8080:80"
	port.Focus()
	return portMapModel{port: port}
}

func (m portMapModel) Init() tea.Cmd { return nil }

func (m portMapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if !portMapRe.MatchString(m.port.Value()) {
				m.err = fmt.Sprintf("Invalid port mapping %q (expected local:remote, e.g. 8080:80)", m.port.Value())
				return m, nil
			}
			m.value = m.port.Value()
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.port, cmd = m.port.Update(msg)
	return m, cmd
}

func (m portMapModel) View() string {
	return fmt.Sprintf("%s\n\n%s\n%s\n%s",
		titleStyle.Render("Port mapping"), m.port.View(), errLine(m.err), hintStyle.Render("enter to continue · esc to cancel"))
}

func errLine(e string) string {
	if e == "" {
		return ""
	}
	return errStyle.Render(e)
}

// PortMapResult extracts the entered port mapping (and whether the user
// cancelled) from a finished tea.Model built with NewPortMapInput.
func PortMapResult(m tea.Model) (value string, cancelled bool) {
	p, _ := m.(portMapModel)
	return p.value, p.cancelled
}
