// Package gcpx wraps gcloud helper commands, including IAP bastion tunnels.
package gcpx

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tdtk/internal/logx"
)

const (
	BastionZone      = "asia-southeast1-a"
	BastionSocksPort = "1080"
)

func BastionProject(env string) (string, error) {
	switch env {
	case "prod":
		return "tech-svc-prod", nil
	case "nonprod":
		return "tech-svc-nonprod", nil
	default:
		return "", fmt.Errorf("unknown bastion env: '%s' (expected: prod|nonprod)", env)
	}
}

func BastionInstance(env string) (string, error) {
	switch env {
	case "prod":
		return "cjx-bastion", nil
	case "nonprod":
		return "tech-bastion", nil
	default:
		return "", fmt.Errorf("unknown bastion env: '%s' (expected: prod|nonprod)", env)
	}
}

func sshTunnelArgs(instance, project string) []string {
	return []string{
		"compute", "ssh", instance,
		"--tunnel-through-iap",
		"--project=" + project,
		"--zone=" + BastionZone,
		"--ssh-flag=-D " + BastionSocksPort + " -q -N",
	}
}

// Bastion opens a foreground SOCKS5 tunnel through the IAP bastion. Blocks.
func Bastion(env string) error {
	project, err := BastionProject(env)
	if err != nil {
		return err
	}
	instance, err := BastionInstance(env)
	if err != nil {
		return err
	}

	logx.Info("Opening SOCKS5 tunnel on localhost:%s via %s (%s)", BastionSocksPort, instance, project)
	cmd := exec.Command("gcloud", sshTunnelArgs(instance, project)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// --- Bastion state (background tunnel management for gke commands) ---

func StateDir() string {
	if d := os.Getenv("TDTK_STATE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "tdtk")
}

func pidFile(env string) string {
	return filepath.Join(StateDir(), "bastion-"+env+".pid")
}

func RememberBastionEnv(env string) error {
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(StateDir(), "last-bastion-env"), []byte(env), 0o644)
}

func portOpen() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+BastionSocksPort, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; Signal(0) checks liveness.
	return proc.Signal(syscallSig0()) == nil
}

// EnsureBastion makes sure a SOCKS5 tunnel is up on localhost:BastionSocksPort
// for the given env, starting it in the background if needed. Blocks until
// the port accepts connections or a 30s timeout is hit.
func EnsureBastion(env string) error {
	stateDir := StateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	pf := pidFile(env)

	if portOpen() {
		logx.Info("Bastion tunnel already up on localhost:%s", BastionSocksPort)
		return nil
	}

	running := false
	if b, err := os.ReadFile(pf); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && processAlive(pid) {
			logx.Info("Bastion tunnel process already running (pid %d), waiting for port...", pid)
			running = true
		}
	}

	if !running {
		project, err := BastionProject(env)
		if err != nil {
			return err
		}
		instance, err := BastionInstance(env)
		if err != nil {
			return err
		}
		logx.Info("Starting bastion tunnel in background for env '%s' via %s (%s)...", env, instance, project)

		logFile, err := os.Create(filepath.Join(stateDir, "bastion-"+env+".log"))
		if err != nil {
			return err
		}
		defer func() { _ = logFile.Close() }()

		cmd := exec.Command("gcloud", sshTunnelArgs(instance, project)...)
		cmd.Stdout, cmd.Stderr = logFile, logFile
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("starting bastion tunnel: %w", err)
		}
		if err := os.WriteFile(pf, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
			return err
		}
		// Detach: don't wait on the child, let it run in the background.
		go func() { _ = cmd.Wait() }()
	}

	for waited := 0; !portOpen(); waited++ {
		if waited >= 30 {
			return fmt.Errorf("timed out waiting for bastion tunnel on localhost:%s (see %s)",
				BastionSocksPort, filepath.Join(stateDir, "bastion-"+env+".log"))
		}
		time.Sleep(1 * time.Second)
	}

	logx.Info("Bastion tunnel ready on localhost:%s", BastionSocksPort)
	return nil
}
