// tdtk is a Go/bubbletea rewrite of the TDTech Toolkit bash CLI: access to
// this org's GCP and GKE resources (IAP bastion tunnels, GKE cluster
// profiles, kubectl).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tdtk/internal/config"
	"tdtk/internal/doctorx"
	"tdtk/internal/gcpx"
	"tdtk/internal/gkex"
	"tdtk/internal/logx"
	"tdtk/internal/tui"
)

func usage() {
	fmt.Print(`TDTech Toolkit - access GCP and K8s tools/services

Setup tools, GCP, GKE (k9s, port-forward), and Doctor are all reachable
from the interactive menu tdtk opens on launch.
`)
}

func requireCmds(cmds ...string) {
	var missing []string
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		logx.Die("Missing required tools: %v", missing)
	}
}

func main() {
	runMenu()
}

// pause waits for the user to acknowledge output before returning to a menu.
func pause() {
	fmt.Println("\npress enter to continue...")
	_, _ = fmt.Scanln()
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// --- Interactive menu ---

func runMenu() {
	for {
		final, err := tea.NewProgram(tui.NewMainMenu()).Run()
		if err != nil {
			logx.Die("menu: %v", err)
		}
		switch tui.MenuResult(final) {
		case tui.MenuSetup:
			runSetup()
		case tui.MenuGCP:
			runGCPMenu()
		case tui.MenuGKE:
			runGKEMenu()
		case tui.MenuDoctor:
			runDoctor()
		case tui.MenuHelp:
			usage()
			pause()
		default:
			return
		}
	}
}

func runSetup() {
	final, err := tea.NewProgram(tui.NewSetupModel()).Run()
	if err != nil {
		logx.Die("setup: %v", err)
	}
	install, missing := tui.SetupResult(final)
	if !install {
		return
	}
	for _, tool := range missing {
		hint := doctorx.InstallHint(tool)
		if hint == "" {
			continue
		}
		logx.Info("Installing %s: %s", tool, hint)
		c := exec.Command("sh", "-c", hint)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			logx.Warn("failed installing %s: %v", tool, err)
		}
	}
	pause()
}

func runDoctor() {
	m := tui.NewDoctorModel()
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		logx.Die("doctor: %v", err)
	}
	if !tui.DoctorOK(final) {
		os.Exit(1)
	}
}

// --- GCP menu ---

func runGCPMenu() {
	for {
		final, err := tea.NewProgram(tui.NewGCPMenu()).Run()
		if err != nil {
			logx.Die("gcp menu: %v", err)
		}
		switch tui.MenuResult(final) {
		case tui.MenuGCPLogin:
			requireCmds("gcloud")
			c := exec.Command("gcloud", "auth", "application-default", "login")
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				logx.Warn("%v", err)
			}
			pause()
		case tui.MenuGCPSetProject:
			runGCPSetProject()
		case tui.MenuGCPBastionBackground:
			runGCPBastionBackground()
		default:
			return
		}
	}
}

func runGCPSetProject() {
	requireCmds("gcloud")
	out, err := exec.Command("gcloud", "projects", "list", "--format=value(projectId)").Output()
	if err != nil {
		logx.Warn("listing projects: %v", err)
		pause()
		return
	}
	projects := nonEmptyLines(string(out))
	if len(projects) == 0 {
		logx.Warn("no projects visible to the current account")
		pause()
		return
	}

	final, err := tea.NewProgram(tui.NewStringSelect("Select a project", projects)).Run()
	if err != nil {
		logx.Die("select project: %v", err)
	}
	project, cancelled := tui.StringSelectResult(final)
	if cancelled || project == "" {
		return
	}

	c := exec.Command("gcloud", "config", "set", "project", project)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		logx.Warn("%v", err)
	}
	pause()
}

func runGCPBastionBackground() {
	requireCmds("gcloud")
	final, err := tea.NewProgram(tui.NewStringSelect("Select bastion env", []string{"prod", "nonprod"})).Run()
	if err != nil {
		logx.Die("select env: %v", err)
	}
	env, cancelled := tui.StringSelectResult(final)
	if cancelled || env == "" {
		return
	}

	if err := gcpx.EnsureBastion(env); err != nil {
		logx.Warn("%v", err)
		pause()
		return
	}
	if err := gcpx.RememberBastionEnv(env); err != nil {
		logx.Warn("%v", err)
	}
	pause()
}

// --- GKE menu ---

func runGKEMenu() {
	requireCmds("kubectl")
	for {
		final, err := tea.NewProgram(tui.NewGKEMenu()).Run()
		if err != nil {
			logx.Die("gke menu: %v", err)
		}
		switch tui.MenuResult(final) {
		case tui.MenuGKEK9s:
			runGKEK9s()
		case tui.MenuGKEPortForward:
			runGKEPortForwardWizard()
		default:
			return
		}
	}
}

// selectProfile prompts for a cluster profile. cancelled is true if the
// user backed out, or if there was nothing to pick from.
func selectProfile() (profile string, cancelled bool) {
	profiles, err := config.Profiles()
	if err != nil {
		logx.Warn("%v", err)
		pause()
		return "", true
	}
	if len(profiles) == 0 {
		logx.Warn("no cluster profiles configured (see config/clusters.conf)")
		pause()
		return "", true
	}

	final, err := tea.NewProgram(tui.NewProfileSelect(profiles)).Run()
	if err != nil {
		logx.Die("select profile: %v", err)
	}
	return tui.ProfileSelectResult(final)
}

// switchToProfile ensures the bastion for profile is up and switches
// kubectl to it, reporting failures to the user.
func switchToProfile(profile string) bool {
	if err := gkex.EnsureBastionForProfile(profile); err != nil {
		logx.Warn("%v", err)
		pause()
		return false
	}
	if err := gkex.Use(profile); err != nil {
		logx.Warn("%v", err)
		pause()
		return false
	}
	return true
}

func runGKEK9s() {
	requireCmds("k9s")
	profile, cancelled := selectProfile()
	if cancelled || profile == "" {
		return
	}
	if !switchToProfile(profile) {
		return
	}
	if err := os.Setenv("HTTPS_PROXY", "socks5://localhost:"+gcpx.BastionSocksPort); err != nil {
		logx.Warn("failed to set HTTPS_PROXY: %v", err)
		pause()
		return
	}
	c := exec.Command("k9s")
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		logx.Warn("k9s: %v", err)
		pause()
	}
}

func runGKEPortForwardWizard() {
	profile, cancelled := selectProfile()
	if cancelled || profile == "" {
		return
	}
	if !switchToProfile(profile) {
		return
	}

	if err := os.Setenv("HTTPS_PROXY", "socks5://localhost:"+gcpx.BastionSocksPort); err != nil {
		logx.Warn("failed to set HTTPS_PROXY: %v", err)
		pause()
		return
	}

	namespaces, err := gkex.ListNamespaces()
	if err != nil {
		logx.Warn("listing namespaces: %v", err)
		pause()
		return
	}
	if len(namespaces) == 0 {
		logx.Warn("no namespaces found")
		pause()
		return
	}
	final, err := tea.NewProgram(tui.NewStringSelect("Select a namespace", namespaces)).Run()
	if err != nil {
		logx.Die("select namespace: %v", err)
	}
	ns, cancelled := tui.StringSelectResult(final)
	if cancelled || ns == "" {
		return
	}

	services, err := gkex.ListServices(ns)
	if err != nil {
		logx.Warn("listing services in '%s': %v", ns, err)
		pause()
		return
	}
	if len(services) == 0 {
		logx.Warn("no services found in namespace '%s'", ns)
		pause()
		return
	}
	final, err = tea.NewProgram(tui.NewStringSelect("Select a service", services)).Run()
	if err != nil {
		logx.Die("select service: %v", err)
	}
	svc, cancelled := tui.StringSelectResult(final)
	if cancelled || svc == "" {
		return
	}

	final, err = tea.NewProgram(tui.NewPortMapInput()).Run()
	if err != nil {
		logx.Die("port map: %v", err)
	}
	portMap, cancelled := tui.PortMapResult(final)
	if cancelled || portMap == "" {
		return
	}

	if err := gkex.PortForwardExec(ns, svc, portMap); err != nil {
		logx.Warn("%v", err)
		pause()
	}
}
