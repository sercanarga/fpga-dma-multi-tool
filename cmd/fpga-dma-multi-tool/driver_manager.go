package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type componentStatus struct {
	Name      string
	Installed bool
	Details   string
}

type rs232Device struct {
	VID          string
	PID          string
	Interface    string
	Service      string
	Name         string
	Manufacturer string
	DriverINF    string
	InstanceID   string
	Serial       string
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
	if !strings.Contains(text, "winusb") &&
		!strings.Contains(text, "libusbk") &&
		!strings.Contains(text, "libusb0") {
		return false
	}
	ft2232h := strings.Contains(text, `vid_0403&pid_6010`) &&
		(!strings.Contains(text, `vid_0403&pid_6010&mi_`) ||
			strings.Contains(text, `vid_0403&pid_6010&mi_00`))
	ft4232h := strings.Contains(text, `vid_0403&pid_6011`) &&
		(!strings.Contains(text, `vid_0403&pid_6011&mi_`) ||
			strings.Contains(text, `vid_0403&pid_6011&mi_00`))
	ft232h := strings.Contains(text, `vid_0403&pid_6014`) &&
		(!strings.Contains(text, `vid_0403&pid_6014&mi_`) ||
			strings.Contains(text, `vid_0403&pid_6014&mi_00`))
	return ft2232h || ft4232h || ft232h
}

func rs232ServiceReady(service string) bool {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "winusb", "libusbk", "libusb0":
		return true
	default:
		return false
	}
}

func rs232CableCandidates(device rs232Device) ([]string, error) {
	description := strings.ToLower(strings.Join(
		[]string{device.Name, device.Manufacturer},
		" ",
	))
	switch strings.ToUpper(strings.TrimSpace(device.PID)) {
	case "6010":
		if strings.Contains(description, "digilent") {
			return []string{"digilent", "rs_dma", "ft2232"}, nil
		}
		return []string{"ft2232", "rs_dma", "digilent"}, nil
	case "6011":
		return []string{"rs_dma", "ft4232"}, nil
	case "6014":
		switch {
		case strings.Contains(description, "hs3"):
			return []string{"digilent_hs3", "rs_dma", "digilent_hs2", "ft232", "digilent_ad"}, nil
		case strings.Contains(description, "hs2"),
			strings.Contains(description, "smt2"):
			return []string{"digilent_hs2", "rs_dma", "digilent_hs3", "ft232", "digilent_ad"}, nil
		case strings.Contains(description, "analog discovery"):
			return []string{"digilent_ad", "rs_dma", "digilent_hs2", "digilent_hs3", "ft232"}, nil
		case strings.Contains(description, "digilent"):
			return []string{"rs_dma", "digilent_hs3", "digilent_hs2", "digilent_ad", "ft232"}, nil
		default:
			return []string{"ft232", "rs_dma", "digilent_hs3", "digilent_hs2", "digilent_ad"}, nil
		}
	default:
		return nil, fmt.Errorf("unsupported FTDI writer PID %s", device.PID)
	}
}

func rs232DeviceArguments(device rs232Device) []string {
	vid := strings.ToUpper(strings.TrimSpace(device.VID))
	if vid == "" {
		vid = "0403"
	}
	arguments := []string{
		"--vid", "0x" + vid,
		"--pid", "0x" + strings.ToUpper(strings.TrimSpace(device.PID)),
		"--ftdi-channel", "0",
	}
	serial := strings.TrimSpace(device.Serial)
	if serial != "" && !strings.ContainsAny(serial, `\&`) {
		arguments = append(arguments, "--ftdi-serial", serial)
	}
	return arguments
}

func rs232DriverINFs(devices []rs232Device) []string {
	unique := make(map[string]bool)
	for _, device := range devices {
		if !rs232ServiceReady(device.Service) {
			continue
		}
		infPath := strings.ReplaceAll(strings.TrimSpace(device.DriverINF), `\`, "/")
		inf := strings.ToLower(filepath.Base(infPath))
		if !strings.HasPrefix(inf, "oem") || !strings.HasSuffix(inf, ".inf") {
			continue
		}
		number := strings.TrimSuffix(strings.TrimPrefix(inf, "oem"), ".inf")
		if _, err := strconv.Atoi(number); err == nil {
			unique[inf] = true
		}
	}
	infNames := make([]string, 0, len(unique))
	for inf := range unique {
		infNames = append(infNames, inf)
	}
	sort.Strings(infNames)
	return infNames
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
