package main

import "testing"

func TestFilterDeviceHistoryEntriesHidesConnectedByDefault(t *testing.T) {
	entries := []deviceHistoryEntry{
		{FriendlyName: "Connected", Present: true},
		{FriendlyName: "Disconnected", Present: false},
	}

	filtered := filterDeviceHistoryEntries(entries, false, "")
	if len(filtered) != 1 || filtered[0].FriendlyName != "Disconnected" {
		t.Fatalf("filtered entries = %#v, want only disconnected device", filtered)
	}

	all := filterDeviceHistoryEntries(entries, true, "")
	if len(all) != 2 {
		t.Fatalf("entries with connected devices enabled = %d, want 2", len(all))
	}
}

func TestFilterDeviceHistoryEntriesSearchesEveryDisplayedIdentity(t *testing.T) {
	entries := []deviceHistoryEntry{
		{
			Enumerator:   "SWD",
			DeviceID:     "WPDBUSENUM",
			InstanceID:   "_??_USBSTOR#DISK",
			Class:        "WPD",
			FriendlyName: "Portable Device",
		},
		{
			Enumerator:   "SWD",
			DeviceID:     "MMDEVAPI",
			InstanceID:   "{0.0.0.00000000}.AUDIO",
			Class:        "AudioEndpoint",
			FriendlyName: "Speakers",
		},
	}

	for query, want := range map[string]string{
		"portable":      "Portable Device",
		"wpd":           "Portable Device",
		"wpdbusenum":    "Portable Device",
		"audioendpoint": "Speakers",
		"speakers":      "Speakers",
		"mmdevapi":      "Speakers",
	} {
		filtered := filterDeviceHistoryEntries(entries, false, query)
		if len(filtered) != 1 || filtered[0].FriendlyName != want {
			t.Fatalf("query %q returned %#v, want %q", query, filtered, want)
		}
	}
}

func TestDeviceHistoryInstanceIDSupportsWindowsPnPEnumerators(t *testing.T) {
	entry := deviceHistoryEntry{
		Enumerator: "SWD",
		DeviceID:   "MMDEVAPI",
		InstanceID: "{0.0.0.00000000}.AUDIO",
	}
	want := `SWD\MMDEVAPI\{0.0.0.00000000}.AUDIO`
	if got := deviceHistoryInstanceID(entry); got != want {
		t.Fatalf("deviceHistoryInstanceID() = %q, want %q", got, want)
	}
}

func TestDeviceEnumRegistryPathAcceptsExactPnPInstances(t *testing.T) {
	for instanceID, want := range map[string]string{
		`PCI\VEN_1234&DEV_5678\1&2&3`: `SYSTEM\CurrentControlSet\Enum\PCI\VEN_1234&DEV_5678\1&2&3`,
		`SWD\WPDBUSENUM\USBSTOR#DISK`: `SYSTEM\CurrentControlSet\Enum\SWD\WPDBUSENUM\USBSTOR#DISK`,
		`HDAUDIO\FUNC_01&VEN_10EC\1`:  `SYSTEM\CurrentControlSet\Enum\HDAUDIO\FUNC_01&VEN_10EC\1`,
	} {
		path, err := deviceEnumRegistryPath(instanceID)
		if err != nil {
			t.Fatalf("deviceEnumRegistryPath(%q) error: %v", instanceID, err)
		}
		if path != want {
			t.Fatalf("registry path = %q, want %q", path, want)
		}
	}

	for _, invalid := range []string{
		`PCI\ONLY_TWO_PARTS`,
		`USB\\INSTANCE`,
		`BAD ENUM\DEVICE\INSTANCE`,
		`SWD\..\INSTANCE`,
		`SWD\DEVICE\..\INSTANCE`,
		`SWD/OTHER\DEVICE\INSTANCE`,
	} {
		if _, err := deviceEnumRegistryPath(invalid); err == nil {
			t.Fatalf("deviceEnumRegistryPath(%q) did not reject invalid ID", invalid)
		}
	}
}
