package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type componentStatus struct {
	Name      string
	Installed bool
	Details   string
}

func setupRequired(statuses []componentStatus) bool {
	for _, status := range statuses {
		if !status.Installed {
			return true
		}
	}
	return false
}

func driverINFMatches(contents []byte, originalINF string) bool {
	needle := strings.TrimSuffix(strings.ToLower(filepath.Base(originalINF)), ".inf")
	return needle != "" && strings.Contains(strings.ToLower(string(contents)), needle)
}

func bundledPath(parts ...string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate the application folder: %w", err)
	}
	pathParts := append([]string{filepath.Dir(executable)}, parts...)
	path := filepath.Join(pathParts...)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("bundled file was not found: %s", path)
	}
	return path, nil
}
