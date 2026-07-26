package main

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestRS232DriverINFMatchesOnlySupportedUSBInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     bool
	}{
		{"dual writer", `DeviceID="USB\VID_0403&PID_6010&MI_00" Service=WinUSB`, true},
		{"dual writer parent using libusbK", `DeviceID="USB\VID_0403&PID_6010" AddService=libusbK`, true},
		{"quad writer", `DeviceID="USB\VID_0403&PID_6011&MI_00" Service=WinUSB`, true},
		{"quad writer parent using libusb0", `DeviceID="USB\VID_0403&PID_6011" Service=libusb0`, true},
		{"ft232h writer", `DeviceID="USB\VID_0403&PID_6014" Service=WinUSB`, true},
		{"wrong dual interface", `DeviceID="USB\VID_0403&PID_6010&MI_01" Service=WinUSB`, false},
		{"wrong interface", `DeviceID="USB\VID_0403&PID_6011&MI_01" Service=WinUSB`, false},
		{"wrong ft232h interface", `DeviceID="USB\VID_0403&PID_6014&MI_01" Service=WinUSB`, false},
		{"manufacturer driver", `DeviceID="USB\VID_0403&PID_6011&MI_00" Service=FTDIBUS`, false},
		{"unrelated FTDI", `DeviceID="USB\VID_0403&PID_6015" Service=WinUSB`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rs232DriverINFMatches([]byte(test.contents)); got != test.want {
				t.Fatalf("rs232DriverINFMatches() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRS232CableCandidatesPreferDetectedDigilentProfile(t *testing.T) {
	tests := []struct {
		name   string
		device rs232Device
		first  string
	}{
		{
			name: "Digilent FT2232H",
			device: rs232Device{
				PID: "6010", Name: "Digilent USB Device",
			},
			first: "digilent",
		},
		{
			name: "Digilent HS3",
			device: rs232Device{
				PID: "6014", Name: "Digilent JTAG-HS3",
			},
			first: "digilent_hs3",
		},
		{
			name: "RS DMA FT4232H",
			device: rs232Device{
				PID: "6011", Name: "Digilent USB Device",
			},
			first: "rs_dma",
		},
		{
			name: "generic FT232H",
			device: rs232Device{
				PID: "6014", Name: "FT232H",
			},
			first: "ft232",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidates, err := rs232CableCandidates(test.device)
			if err != nil {
				t.Fatal(err)
			}
			if len(candidates) == 0 || candidates[0] != test.first {
				t.Fatalf("candidates = %v, want first %q", candidates, test.first)
			}
		})
	}
}

func TestRS232DeviceArgumentsUseStableSerialOnly(t *testing.T) {
	withSerial := rs232DeviceArguments(rs232Device{PID: "6010", Serial: "D12345"})
	if got := strings.Join(withSerial, " "); !strings.Contains(got, "--ftdi-serial D12345") {
		t.Fatalf("arguments = %v", withSerial)
	}
	withoutSerial := rs232DeviceArguments(rs232Device{PID: "6010", Serial: "6&12345&0&0000"})
	if got := strings.Join(withoutSerial, " "); strings.Contains(got, "--ftdi-serial") {
		t.Fatalf("location-based instance ID was used as a serial: %v", withoutSerial)
	}
	customVID := rs232DeviceArguments(rs232Device{VID: "1234", PID: "6011"})
	if got := strings.Join(customVID, " "); !strings.Contains(got, "--vid 0x1234") {
		t.Fatalf("detected VID was not used: %v", customVID)
	}
}

func TestRS232DriverINFsUseExactReadyDevicePackages(t *testing.T) {
	devices := []rs232Device{
		{Service: "WinUSB", DriverINF: `C:\Windows\INF\oem42.inf`},
		{Service: "libusbK", DriverINF: "OEM7.INF"},
		{Service: "FTDIBUS", DriverINF: "oem9.inf"},
		{Service: "WinUSB", DriverINF: "ftdibus.inf"},
		{Service: "WinUSB", DriverINF: "oem42.inf"},
	}
	got := rs232DriverINFs(devices)
	want := []string{"oem42.inf", "oem7.inf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rs232DriverINFs() = %v, want %v", got, want)
	}
}
