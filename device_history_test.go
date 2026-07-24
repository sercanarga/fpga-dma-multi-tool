package main

import "testing"

func TestFilterDeviceHistoryEntriesHidesConnectedByDefault(t *testing.T) {
	entries := []deviceHistoryEntry{
		{FriendlyName: "Connected", Present: true},
		{FriendlyName: "Disconnected", Present: false},
	}

	filtered := filterDeviceHistoryEntries(entries, false)
	if len(filtered) != 1 || filtered[0].FriendlyName != "Disconnected" {
		t.Fatalf("filtered entries = %#v, want only disconnected device", filtered)
	}

	all := filterDeviceHistoryEntries(entries, true)
	if len(all) != 2 {
		t.Fatalf("entries with connected devices enabled = %d, want 2", len(all))
	}
}

func TestDeviceHistoryInstanceID(t *testing.T) {
	entry := deviceHistoryEntry{
		Type:       "PCI",
		HardwareID: "VEN_1102&DEV_0012",
		InstanceID: "8&24fffb57&4&12",
	}
	want := `PCI\VEN_1102&DEV_0012\8&24fffb57&4&12`
	if got := deviceHistoryInstanceID(entry); got != want {
		t.Fatalf("deviceHistoryInstanceID() = %q, want %q", got, want)
	}
}

func TestDeviceEnumRegistryPathIsLimitedToExactPCIOrUSBInstance(t *testing.T) {
	path, err := deviceEnumRegistryPath(`PCI\VEN_1234&DEV_5678\1&2&3`)
	if err != nil {
		t.Fatal(err)
	}
	if path != `SYSTEM\CurrentControlSet\Enum\PCI\VEN_1234&DEV_5678\1&2&3` {
		t.Fatalf("registry path = %q", path)
	}
	for _, invalid := range []string{
		`ROOT\DEVICE\0000`,
		`PCI\ONLY_TWO_PARTS`,
		`USB\\INSTANCE`,
	} {
		if _, err := deviceEnumRegistryPath(invalid); err == nil {
			t.Fatalf("deviceEnumRegistryPath(%q) did not reject invalid ID", invalid)
		}
	}
}
