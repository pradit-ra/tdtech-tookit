package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"tdtk/internal/doctorx"
)

type setupDoneMsg struct{ tools []doctorx.ToolCheck }

type setupModel struct {
	spinner spinner.Model
	loading bool
	tools   []doctorx.ToolCheck
	install bool
}

// NewSetupModel checks required tools and offers to install the missing
// ones with brew.
func NewSetupModel() tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return setupModel{spinner: s, loading: true}
}

func checkSetupTools() tea.Msg {
	time.Sleep(300 * time.Millisecond)
	return setupDoneMsg{tools: doctorx.CheckTools()}
}

func (m setupModel) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, checkSetupTools) }

func (m setupModel) missing() []string {
	var missing []string
	for _, t := range m.tools {
		if !t.Found {
			missing = append(missing, t.Tool)
		}
	}
	return missing
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "i":
			if len(m.missing()) > 0 {
				m.install = true
			}
			return m, tea.Quit
		case "q", "esc", "ctrl+c", "enter":
			return m, tea.Quit
		}
		return m, nil
	case setupDoneMsg:
		m.loading = false
		m.tools = msg.tools
		return m, nil
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m setupModel) View() string {
	if m.loading {
		return fmt.Sprintf("%s Checking tools...\n", m.spinner.View())
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tools"))
	b.WriteString("\n")
	missing := m.missing()
	for _, t := range m.tools {
		if t.Found {
			fmt.Fprintf(&b, "  %s %s (%s)\n", okStyle.Render("✔"), t.Tool, dimStyle.Render(t.Path))
		} else {
			fmt.Fprintf(&b, "  %s %s %s\n", badStyle.Render("✘"), t.Tool, dimStyle.Render(doctorx.InstallHint(t.Tool)))
		}
	}

	b.WriteString("\n")
	if len(missing) == 0 {
		b.WriteString(okStyle.Render("All required tools are installed.") + "\n")
		b.WriteString(dimStyle.Render("press q to exit"))
	} else {
		b.WriteString(dimStyle.Render("press i to install missing tools with brew · q to skip"))
	}

	return b.String()
}

// SetupResult reports whether the user asked to install, and which tools
// are missing, from a finished tea.Model built with NewSetupModel.
func SetupResult(m tea.Model) (install bool, missing []string) {
	s, _ := m.(setupModel)
	return s.install, s.missing()
}
