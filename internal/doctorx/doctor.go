// Package doctorx runs environment diagnostics: required tools and the
// GKE clusters config.
package doctorx

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"tdtk/internal/config"
)

var requiredTools = []string{"gcloud", "kubectl", "kubectx", "k9s"}

// InstallHint returns the brew command to install a required tool.
func InstallHint(tool string) string {
	switch tool {
	case "gcloud":
		return "brew install --cask google-cloud-sdk"
	case "kubectl":
		return "brew install kubectl"
	case "kubectx":
		return "brew install kubectx"
	case "k9s":
		return "brew install k9s"
	default:
		return ""
	}
}

type ToolCheck struct {
	Tool  string
	Path  string
	Found bool
}

func CheckTools() []ToolCheck {
	checks := make([]ToolCheck, 0, len(requiredTools))
	for _, tool := range requiredTools {
		path, err := exec.LookPath(tool)
		checks = append(checks, ToolCheck{Tool: tool, Path: path, Found: err == nil})
	}
	return checks
}

func MissingHints(checks []ToolCheck) []string {
	var hints []string
	for _, c := range checks {
		if !c.Found {
			hints = append(hints, fmt.Sprintf("%s: %s", c.Tool, InstallHint(c.Tool)))
		}
	}
	return hints
}

type ConfigIssue struct {
	Line    int
	Message string
}

type ConfigCheck struct {
	File   string
	Found  bool
	Count  int
	Issues []ConfigIssue
}

func CheckClustersConfig() ConfigCheck {
	check := ConfigCheck{File: config.File()}

	lines, err := config.Lines()
	if err != nil {
		return check // Found stays false
	}
	check.Found = true

	// Re-scan raw lines (with line numbers) for structural validation,
	// since config.Lines() already strips comments/blanks.
	rawLines, lineNumbers := rawNonCommentLines(check.File)
	for i, line := range rawLines {
		lineno := lineNumbers[i]
		fields := splitColon(line)
		if len(fields) != 5 {
			check.Issues = append(check.Issues, ConfigIssue{lineno, "malformed, expected 5 ':'-separated fields"})
			continue
		}

		switch fields[2] {
		case "zone", "region":
		default:
			check.Issues = append(check.Issues, ConfigIssue{lineno, fmt.Sprintf("invalid location_type '%s' (expected zone|region)", fields[2])})
		}

		if config.BastionEnv(fields[0]) == "" {
			check.Issues = append(check.Issues, ConfigIssue{lineno, fmt.Sprintf("profile '%s' doesn't start with 'prod' or 'nonprod' (bastion env can't be determined)", fields[0])})
		}
		check.Count++
	}
	_ = lines

	return check
}

func rawNonCommentLines(file string) (lines []string, lineNumbers []int) {
	f, err := os.Open(file)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = f.Close() }()

	lineno := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineno++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
		lineNumbers = append(lineNumbers, lineno)
	}
	return lines, lineNumbers
}

func splitColon(line string) []string {
	return strings.Split(line, ":")
}
