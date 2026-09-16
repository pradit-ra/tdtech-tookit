// Package gkex wraps kubectl/kubectx helper commands, driven by clusters.conf.
package gkex

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"tdtk/internal/config"
	"tdtk/internal/gcpx"
	"tdtk/internal/logx"
)

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func output(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Use fetches gcloud credentials for the cluster profile and switches the
// kubectl context to it (renamed to the profile name via kubectx).
func Use(profile string) error {
	p, err := config.Lookup(profile)
	if err != nil {
		return err
	}

	var locFlag string
	switch p.LocationType {
	case "zone":
		locFlag = "--zone"
	case "region":
		locFlag = "--region"
	default:
		return fmt.Errorf("invalid location type '%s' for profile '%s' (expected: zone|region)", p.LocationType, profile)
	}

	logx.Info("Fetching credentials for cluster '%s' (project=%s, %s=%s)", p.Cluster, p.Project, p.LocationType, p.Location)
	if err := run("gcloud", "container", "clusters", "get-credentials", p.Cluster,
		"--project="+p.Project, locFlag, p.Location); err != nil {
		return err
	}

	rawContext, err := output("kubectl", "config", "current-context")
	if err != nil {
		return err
	}

	if rawContext != profile {
		if _, err := exec.Command("kubectl", "config", "get-contexts", profile).Output(); err == nil {
			_ = exec.Command("kubectl", "config", "delete-context", profile).Run()
		}
		if err := exec.Command("kubectx", profile+"="+rawContext).Run(); err != nil {
			return err
		}
	}

	if err := exec.Command("kubectx", profile).Run(); err != nil {
		return err
	}

	current, err := output("kubectl", "config", "current-context")
	if err != nil {
		return err
	}
	logx.Info("Active context: %s", current)
	return nil
}

// EnsureBastionForProfile resolves the bastion env for profile and makes
// sure the tunnel/proxy is set up before kubectl-touching commands run.
func EnsureBastionForProfile(profile string) error {
	env := config.BastionEnv(profile)
	if env == "" {
		logx.Warn("Could not determine bastion env (prod|nonprod) for profile '%s' (expected name to start with 'prod' or 'nonprod'); skipping tunnel/proxy setup.", profile)
		return nil
	}

	if err := gcpx.EnsureBastion(env); err != nil {
		return err
	}
	return gcpx.RememberBastionEnv(env)
}

func PortForwardExec(ns, svc, portMap string) error {
	logx.Info("Port-forwarding svc/%s in namespace '%s' (%s)... (Ctrl-C to stop)", svc, ns, portMap)
	return run("kubectl", "port-forward", "-n", ns, "svc/"+svc, portMap)
}

const jsonNames = `{range .items[*]}{.metadata.name}{"\n"}{end}`

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// ListNamespaces returns the names of all namespaces in the current context.
func ListNamespaces() ([]string, error) {
	out, err := output("kubectl", "get", "namespaces", "-o", "jsonpath="+jsonNames)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// ListServices returns the names of all services in the given namespace.
func ListServices(ns string) ([]string, error) {
	out, err := output("kubectl", "get", "svc", "-n", ns, "-o", "jsonpath="+jsonNames)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}
