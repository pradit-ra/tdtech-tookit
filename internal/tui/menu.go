package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MenuChoice identifies which item a choice menu (main, GCP, or GKE) picked.
// MenuNone means the user backed out / quit without picking anything.
type MenuChoice int

const (
	MenuNone MenuChoice = iota

	// Main menu.
	MenuSetup
	MenuGCP
	MenuGKE
	MenuDoctor
	MenuHelp

	// GCP submenu.
	MenuGCPLogin
	MenuGCPSetProject
	MenuGCPBastionBackground

	// GKE submenu.
	MenuGKEK9s
	MenuGKEPortForward
)

type choiceItem struct {
	title, desc string
	choice      MenuChoice
}

func (i choiceItem) Title() string       { return i.title }
func (i choiceItem) Description() string { return i.desc }
func (i choiceItem) FilterValue() string { return i.title }

type choiceMenuModel struct {
	list   list.Model
	choice MenuChoice
}

var (
	accentPink     = lipgloss.AdaptiveColor{Light: "#D93A9A", Dark: "212"}
	accentCyan     = lipgloss.AdaptiveColor{Light: "#0072A3", Dark: "87"}
	textNormal     = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "255"}
	textDimmed     = lipgloss.AdaptiveColor{Light: "#5A5A5A", Dark: "245"}
	titleFg        = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}
	menuTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(titleFg).Background(accentPink).Padding(0, 1)
	statusStyle    = lipgloss.NewStyle().Foreground(accentCyan)
	itemNormal     = lipgloss.NewStyle().Foreground(textNormal)
	itemDimmed     = lipgloss.NewStyle().Foreground(textDimmed)
	itemSelected   = lipgloss.NewStyle().Bold(true).Foreground(accentCyan).BorderStyle(lipgloss.NormalBorder()).BorderForeground(accentCyan).BorderLeft(true).Padding(0, 0, 0, 1)
)

func newChoiceMenu(title string, items []choiceItem) tea.Model {
	li := make([]list.Item, len(items))
	for i, it := range items {
		li[i] = it
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalTitle = itemNormal
	delegate.Styles.NormalDesc = itemDimmed
	delegate.Styles.SelectedTitle = itemSelected
	delegate.Styles.SelectedDesc = itemSelected.Foreground(accentCyan)

	l := list.New(li, delegate, 64, 16)
	l.Title = title
	l.Styles.Title = menuTitleStyle
	l.Styles.StatusBar = statusStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return choiceMenuModel{list: l}
}

func (m choiceMenuModel) Init() tea.Cmd { return nil }

func (m choiceMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "enter":
			if sel, ok := m.list.SelectedItem().(choiceItem); ok {
				m.choice = sel.choice
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m choiceMenuModel) View() string { return m.list.View() }

// NewMainMenu builds tdtk's top-level interactive menu.
func NewMainMenu() tea.Model {
	return newChoiceMenu("TDTech Toolkit", []choiceItem{
		{"Setup tools", "Check and install required CLI tools (gcloud, kubectl, kubectx, k9s)", MenuSetup},
		{"GCP", "Log in, set project, run the IAP bastion tunnel", MenuGCP},
		{"GKE", "Browse clusters with k9s, or port-forward a service", MenuGKE},
		{"Doctor", "Check all tools and config", MenuDoctor},
		{"Help", "Show CLI usage", MenuHelp},
	})
}

// NewGCPMenu builds the GCP submenu.
func NewGCPMenu() tea.Model {
	return newChoiceMenu("GCP", []choiceItem{
		{"Login (application default)", "gcloud auth application-default login", MenuGCPLogin},
		{"Set project", "Pick a project and set it as the active gcloud config", MenuGCPSetProject},
		{"Run bastion (background)", "Start the IAP SOCKS5 tunnel in the background", MenuGCPBastionBackground},
	})
}

// NewGKEMenu builds the GKE submenu.
func NewGKEMenu() tea.Model {
	return newChoiceMenu("GKE", []choiceItem{
		{"k9s", "Switch to a cluster profile and open k9s", MenuGKEK9s},
		{"Port forward", "Switch to a cluster profile, then pick a namespace/service/port", MenuGKEPortForward},
	})
}

// MenuResult extracts the picked item (MenuNone if the user backed out or
// quit) from a finished tea.Model built with NewMainMenu, NewGCPMenu, or
// NewGKEMenu.
func MenuResult(m tea.Model) MenuChoice {
	c, _ := m.(choiceMenuModel)
	return c.choice
}
