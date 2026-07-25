package main

import "testing"

func TestDriverINFMatchesOriginalPackageName(t *testing.T) {
	tests := []struct {
		name        string
		contents    string
		originalINF string
		want        bool
	}{
		{
			name:        "WCH",
			contents:    `[SourceDisksFiles]\nCH341WDM.SYS=1`,
			originalINF: "CH341WDM.INF",
			want:        true,
		},
		{
			name:        "FTDI",
			contents:    `[Strings]\nServiceName="ftdibus3"`,
			originalINF: "ftdibus3.inf",
			want:        true,
		},
		{
			name:        "other package",
			contents:    `[Strings]\nServiceName="usbser"`,
			originalINF: "ftdibus3.inf",
			want:        false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := driverINFMatches([]byte(test.contents), test.originalINF); got != test.want {
				t.Fatalf("driverINFMatches() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSetupRequiredWhenAnyDriverIsMissing(t *testing.T) {
	installed := []componentStatus{
		{Name: "CH347 driver", Installed: true},
		{Name: "FTDI D3XX driver", Installed: true},
		{Name: "RS232 writer driver", Installed: true},
	}
	if setupRequired(installed) {
		t.Fatal("setupRequired() = true when all drivers are installed")
	}
	installed[1].Installed = false
	if !setupRequired(installed) {
		t.Fatal("setupRequired() = false when a driver is missing")
	}
}

func TestProgrammingCableNames(t *testing.T) {
	tests := map[string]string{
		"":        autoProgrammingCable,
		"auto":    autoProgrammingCable,
		"CH347":   directCH347ProgrammingCable,
		"rs232":   rs232ProgrammingCable,
		" RS232 ": rs232ProgrammingCable,
	}
	for input, want := range tests {
		got, err := programmingCableFromName(input)
		if err != nil {
			t.Fatalf("programmingCableFromName(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("programmingCableFromName(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := programmingCableFromName("serial"); err == nil {
		t.Fatal("unsupported programmer name was accepted")
	}
}

func TestRS232DriverINFMatchesOnlySupportedWinUSBInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     bool
	}{
		{"quad writer", `DeviceID="USB\VID_0403&PID_6011&MI_00" Service=WinUSB`, true},
		{"ft232h writer", `DeviceID="USB\VID_0403&PID_6014" Service=WinUSB`, true},
		{"wrong interface", `DeviceID="USB\VID_0403&PID_6011&MI_01" Service=WinUSB`, false},
		{"wrong ft232h interface", `DeviceID="USB\VID_0403&PID_6014&MI_01" Service=WinUSB`, false},
		{"manufacturer driver", `DeviceID="USB\VID_0403&PID_6011&MI_00" Service=FTDIBUS`, false},
		{"unrelated FTDI", `DeviceID="USB\VID_0403&PID_6010&MI_00" Service=WinUSB`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rs232DriverINFMatches([]byte(test.contents)); got != test.want {
				t.Fatalf("rs232DriverINFMatches() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRS232WriterCableMapping(t *testing.T) {
	tests := map[string]string{"6011": "ft4232", "6014": "ft232", " 6014 ": "ft232"}
	for pid, want := range tests {
		got, err := rs232CableForPID(pid)
		if err != nil {
			t.Fatalf("rs232CableForPID(%q) error: %v", pid, err)
		}
		if got != want {
			t.Fatalf("rs232CableForPID(%q) = %q, want %q", pid, got, want)
		}
	}
	if _, err := rs232CableForPID("6010"); err == nil {
		t.Fatal("unsupported FTDI PID was accepted")
	}
}
