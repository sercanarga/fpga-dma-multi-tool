package main

import (
	"fmt"
	"strings"
)

type deviceHistoryEntry struct {
	Type         string   `json:"Type"`
	HardwareID   string   `json:"HardwareId"`
	InstanceID   string   `json:"InstanceId"`
	FriendlyName string   `json:"FriendlyName"`
	Driver       string   `json:"Driver"`
	ControlSets  []string `json:"ControlSets"`
	Present      bool     `json:"Present"`
}

type deviceHistoryCleanupResult struct {
}

func filterDeviceHistoryEntries(
	entries []deviceHistoryEntry,
	showConnected bool,
) []deviceHistoryEntry {
	filtered := make([]deviceHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if showConnected || !entry.Present {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func deviceHistoryInstanceID(device deviceHistoryEntry) string {
	return strings.Join([]string{device.Type, device.HardwareID, device.InstanceID}, `\`)
}

func deviceEnumRegistryPath(instanceID string) (string, error) {
	parts := strings.SplitN(instanceID, `\`, 3)
	if len(parts) != 3 ||
		(parts[0] != "PCI" && parts[0] != "USB") ||
		parts[1] == "" ||
		parts[2] == "" {
		return "", fmt.Errorf("invalid PCI or USB device instance ID")
	}
	return `SYSTEM\CurrentControlSet\Enum\` + instanceID, nil
}
