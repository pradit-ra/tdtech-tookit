// Package config loads GKE cluster profiles from config/clusters.conf.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile mirrors one line of clusters.conf:
// profile:project:location_type:location:cluster
type Profile struct {
	Name         string
	Project      string
	LocationType string // "zone" or "region"
	Location     string
	Cluster      string
}

// File resolves the clusters config path: $TDTK_CLUSTERS_FILE, or
// config/clusters.conf next to the running binary.
func File() string {
	if f := os.Getenv("TDTK_CLUSTERS_FILE"); f != "" {
		return f
	}
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), "config", "clusters.conf")
	}
	return filepath.Join("config", "clusters.conf")
}

// Lines returns raw non-comment, non-blank lines from the clusters config,
// each still ":"-separated, alongside their 1-based line number in the file.
func Lines() ([]string, error) {
	file := File()
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("clusters config not found: %s", file)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func parseLine(line string) (Profile, error) {
	fields := strings.Split(line, ":")
	if len(fields) != 5 {
		return Profile{}, fmt.Errorf("malformed line %q: expected 5 ':'-separated fields", line)
	}
	return Profile{
		Name:         fields[0],
		Project:      fields[1],
		LocationType: fields[2],
		Location:     fields[3],
		Cluster:      fields[4],
	}, nil
}

// Profiles returns all cluster profiles defined in the config.
func Profiles() ([]Profile, error) {
	lines, err := Lines()
	if err != nil {
		return nil, err
	}
	profiles := make([]Profile, 0, len(lines))
	for _, line := range lines {
		p, err := parseLine(line)
		if err != nil {
			continue // malformed lines are reported by doctor, not here
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// Lookup finds a profile by name, erroring if it doesn't exist.
func Lookup(name string) (Profile, error) {
	profiles, err := Profiles()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("unknown cluster profile: '%s' (see: tdtk gke profiles)", name)
}

// BastionEnv derives the bastion env (prod|nonprod) from a profile name
// prefix (e.g. "prod-a", "nonprod-b"). Returns "" if it doesn't match.
func BastionEnv(profile string) string {
	switch {
	case profile == "prod" || strings.HasPrefix(profile, "prod-"):
		return "prod"
	case profile == "nonprod" || strings.HasPrefix(profile, "nonprod-"):
		return "nonprod"
	default:
		return ""
	}
}
