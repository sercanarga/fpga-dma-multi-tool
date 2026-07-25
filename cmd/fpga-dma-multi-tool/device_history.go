package main

import (
	"fmt"
	"regexp"
	"strings"
)

type deviceHistoryEntry struct {
	Enumerator   string `json:"Enumerator"`
	DeviceID     string `json:"DeviceId"`
	InstanceID   string `json:"InstanceId"`
	Class        string `json:"Class"`
	FriendlyName string `json:"FriendlyName"`
	Present      bool   `json:"Present"`
}

var deviceEnumeratorPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func filterDeviceHistoryEntries(
	entries []deviceHistoryEntry,
	showConnected bool,
	query string,
) []deviceHistoryEntry {
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]deviceHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if !showConnected && entry.Present {
			continue
		}
		if needle != "" {
			haystack := strings.ToLower(strings.Join([]string{
				entry.Class,
				entry.FriendlyName,
				entry.Enumerator,
				entry.DeviceID,
				entry.InstanceID,
			}, "\n"))
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func deviceHistoryInstanceID(device deviceHistoryEntry) string {
	return strings.Join(
		[]string{device.Enumerator, device.DeviceID, device.InstanceID},
		`\`,
	)
}

func deviceEnumRegistryPath(instanceID string) (string, error) {
	parts := strings.SplitN(instanceID, `\`, 3)
	if len(parts) != 3 ||
		!deviceEnumeratorPattern.MatchString(parts[0]) ||
		strings.Contains(parts[0], `..`) ||
		parts[1] == "" ||
		parts[2] == "" ||
		strings.Contains(parts[1], `/`) ||
		strings.Contains(parts[2], `/`) ||
		strings.Contains(parts[2], `\`) ||
		strings.Contains(parts[1], `..`) ||
		strings.Contains(parts[2], `..`) ||
		strings.ContainsAny(parts[1], "\x00\r\n") ||
		strings.ContainsAny(parts[2], "\x00\r\n") {
		return "", fmt.Errorf("invalid Windows PnP device instance ID")
	}
	return `SYSTEM\CurrentControlSet\Enum\` + instanceID, nil
}
