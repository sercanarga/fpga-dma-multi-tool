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

func programmingCableFromName(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return autoProgrammingCable, nil
	case "ch347":
		return directCH347ProgrammingCable, nil
	case "rs232":
		return rs232ProgrammingCable, nil
	default:
		return "", fmt.Errorf("unsupported programmer %q; use auto, ch347, or rs232", name)
	}
}

func rs232DriverINFMatches(contents []byte) bool {
	text := strings.ToLower(string(contents))
	if !strings.Contains(text, "winusb") {
		return false
	}
	ft232h := strings.Contains(text, `vid_0403&pid_6014`) &&
		(!strings.Contains(text, `vid_0403&pid_6014&mi_`) ||
			strings.Contains(text, `vid_0403&pid_6014&mi_00`))
	return strings.Contains(text, `vid_0403&pid_6011&mi_00`) || ft232h
}

func rs232CableForPID(pid string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(pid)) {
	case "6011":
		return "ft4232", nil
	case "6014":
		return "ft232", nil
	default:
		return "", fmt.Errorf("unsupported RS232 writer PID %s", pid)
	}
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
