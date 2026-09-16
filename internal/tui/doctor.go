package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tdtk/internal/doctorx"
)

var (
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	badStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

type doctorDoneMsg struct {
	tools []doctorx.ToolCheck
	cfg   doctorx.ConfigCheck
}

type doctorModel struct {
	spinner spinner.Model
	loading bool
	done    doctorDoneMsg
	// OK reports whether every check passed, valid once loading is false.
	OK bool
}

func NewDoctorModel() tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	return doctorModel{spinner: s, loading: true}
}

func runChecks() tea.Msg {
	time.Sleep(300 * time.Millisecond) // let the spinner render at least one frame
	return doctorDoneMsg{
		tools: doctorx.CheckTools(),
		cfg:   doctorx.CheckClustersConfig(),
	}
}

func (m doctorModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runChecks)
}

func (m doctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "enter" || msg.String() == "esc" {
			return m, tea.Quit
		}
		return m, nil
	case doctorDoneMsg:
		m.loading = false
		m.done = msg
		m.OK = allOK(msg)
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

// DoctorOK extracts whether all checks passed from a finished tea.Model.
func DoctorOK(m tea.Model) bool {
	d, _ := m.(doctorModel)
	return d.OK
}

func allOK(d doctorDoneMsg) bool {
	for _, t := range d.tools {
		if !t.Found {
			return false
		}
	}
	return d.cfg.Found && len(d.cfg.Issues) == 0 && d.cfg.Count > 0
}

func (m doctorModel) View() string {
	if m.loading {
		return fmt.Sprintf("%s Checking environment...\n", m.spinner.View())
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tools"))
	b.WriteString("\n")
	var missing []string
	for _, t := range m.done.tools {
		if t.Found {
			fmt.Fprintf(&b, "  %s %s (%s)\n", okStyle.Render("✔"), t.Tool, dimStyle.Render(t.Path))
		} else {
			fmt.Fprintf(&b, "  %s %s\n", badStyle.Render("✘"), t.Tool)
			missing = append(missing, t.Tool)
		}
	}
	if len(missing) > 0 {
		b.WriteString("\n" + badStyle.Render("Missing tools:") + " install with:\n")
		for _, hint := range doctorx.MissingHints(m.done.tools) {
			b.WriteString("  " + hint + "\n")
		}
	}

	b.WriteString("\n" + titleStyle.Render("Clusters config"))
	b.WriteString("\n")
	cfg := m.done.cfg
	switch {
	case !cfg.Found:
		fmt.Fprintf(&b, "  %s not found (%s)\n", badStyle.Render("✘"), cfg.File)
	case len(cfg.Issues) > 0:
		for _, issue := range cfg.Issues {
			fmt.Fprintf(&b, "  %s line %d: %s\n", badStyle.Render("✘"), issue.Line, issue.Message)
		}
	case cfg.Count == 0:
		fmt.Fprintf(&b, "  %s no cluster profiles defined in %s\n", badStyle.Render("✘"), cfg.File)
	default:
		fmt.Fprintf(&b, "  %s %d profile(s) found (%s)\n", okStyle.Render("✔"), cfg.Count, cfg.File)
	}

	b.WriteString("\n")
	if m.OK {
		b.WriteString(okStyle.Render("All checks passed.") + "\n")
	} else {
		b.WriteString(badStyle.Render("Some checks failed.") + "\n")
	}
	b.WriteString(dimStyle.Render("press q to exit"))

	return b.String()
}
