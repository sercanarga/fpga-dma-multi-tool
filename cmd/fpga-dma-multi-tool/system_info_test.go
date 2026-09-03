package main

import (
	"strings"
	"testing"
)

func boolPointer(value bool) *bool { return &value }
func intPointer(value int) *int    { return &value }

func TestDecodeSystemInfoJSONSeparatesCapabilityConfigurationAndRuntime(t *testing.T) {
	encoded := []byte(`{
		"processorName":"AMD Ryzen Test",
		"processorManufacturer":"AuthenticAMD",
		"systemManufacturer":"Board Vendor",
		"systemModel":"Board Model",
		"biosManufacturer":"Firmware Vendor",
		"biosVersion":"1.2.3",
		"windowsCaption":"Microsoft Windows 11 Pro",
		"windowsVersion":"10.0.26200",
		"windowsBuild":"26200",
		"firmwareType":2,
		"virtualizationSupported":false,
		"virtualizationEnabled":true,
		"hypervisorPresent":true,
		"dmaProtectionAvailable":true,
		"deviceGuardAvailable":true,
		"vbsStatus":2,
		"secureBootEnabled":true,
		"memoryIntegrityConfigured":true,
		"memoryIntegrityRunning":true,
		"credentialGuardConfigured":true,
		"credentialGuardRunning":false,
		"secureLaunchConfigured":true,
		"secureLaunchRunning":false,
		"tpmPresent":true,
		"tpmReady":true,
		"tpmSpecVersion":"2.0, 0, 1.59",
		"pcieLinks":[{"Name":"DMA adapter","InstanceID":"PCI\\VEN_1234","CurrentWidth":4,"MaximumWidth":4}]
	}`)
	snapshot, err := decodeSystemInfoJSON(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProcessorName != "AMD Ryzen Test" {
		t.Fatalf("processor = %q", snapshot.ProcessorName)
	}
	if snapshot.SystemSummary != "Board Vendor Board Model" {
		t.Fatalf("system = %q", snapshot.SystemSummary)
	}
	if snapshot.FirmwareSummary != "Firmware Vendor 1.2.3" {
		t.Fatalf("firmware = %q", snapshot.FirmwareSummary)
	}
	if !strings.Contains(snapshot.WindowsSummary, "10.0.26200") {
		t.Fatalf("Windows = %q", snapshot.WindowsSummary)
	}

	want := []struct {
		name  string
		state systemInfoState
	}{
		{"Firmware / BIOS mode", systemInfoUEFI},
		{"Secure Boot", systemInfoOn},
		{"AMD-V / SVM", systemInfoOn},
		{"Windows hypervisor", systemInfoRunning},
		{"Kernel DMA protection", systemInfoAvailable},
		{"Virtualization-based security", systemInfoRunning},
		{"Core isolation / Memory integrity", systemInfoRunning},
		{"Credential Guard", systemInfoConfigured},
		{"System Guard Secure Launch", systemInfoConfigured},
		{"Trusted Platform Module", systemInfoReady},
	}
	if len(snapshot.Features) != len(want) {
		t.Fatalf("feature count = %d, want %d", len(snapshot.Features), len(want))
	}
	for index, expected := range want {
		got := snapshot.Features[index]
		if got.Name != expected.name || got.State != expected.state {
			t.Fatalf("feature %d = %#v, want %q/%q", index, got, expected.name, expected.state)
		}
	}
	if len(snapshot.PCIeLinks) != 1 || snapshot.PCIeLinks[0].CurrentWidth != 4 {
		t.Fatalf("PCIe links = %#v", snapshot.PCIeLinks)
	}
}

func TestBuildSystemInfoSnapshotHandlesLegacyBIOSAndDisabledFeatures(t *testing.T) {
	snapshot := buildSystemInfoSnapshot(rawSystemInfo{
		ProcessorManufacturer:   "GenuineIntel",
		FirmwareType:            intPointer(1),
		VirtualizationSupported: boolPointer(true),
		VirtualizationEnabled:   boolPointer(false),
		HypervisorPresent:       boolPointer(false),
		DMAProtectionAvailable:  boolPointer(false),
		DeviceGuardAvailable:    true,
		VBSStatus:               intPointer(0),
		SecureBootEnabled:       boolPointer(true),
		MemoryIntegrityConfig:   boolPointer(false),
		MemoryIntegrityRunning:  boolPointer(false),
		CredentialGuardConfig:   boolPointer(false),
		CredentialGuardRunning:  boolPointer(false),
		SecureLaunchConfig:      boolPointer(false),
		SecureLaunchRunning:     boolPointer(false),
		TPMPresent:              boolPointer(false),
	})
	want := []struct {
		index int
		name  string
		state systemInfoState
	}{
		{0, "Firmware / BIOS mode", systemInfoLegacy},
		{1, "Secure Boot", systemInfoUnsupported},
		{2, "Intel VT-x", systemInfoOff},
		{3, "Windows hypervisor", systemInfoOff},
		{4, "Kernel DMA protection", systemInfoUnavailable},
		{5, "Virtualization-based security", systemInfoOff},
		{6, "Core isolation / Memory integrity", systemInfoOff},
		{7, "Credential Guard", systemInfoOff},
		{8, "System Guard Secure Launch", systemInfoOff},
		{9, "Trusted Platform Module", systemInfoNotPresent},
	}
	for _, expected := range want {
		got := snapshot.Features[expected.index]
		if got.Name != expected.name || got.State != expected.state {
			t.Fatalf("feature %d = %#v, want %q/%q", expected.index, got, expected.name, expected.state)
		}
	}
}

func TestBuildSystemInfoSnapshotDoesNotMisreadHypervisorMaskedCPUFlags(t *testing.T) {
	snapshot := buildSystemInfoSnapshot(rawSystemInfo{
		ProcessorManufacturer:   "AuthenticAMD",
		VirtualizationSupported: boolPointer(false),
		VirtualizationEnabled:   boolPointer(false),
		HypervisorPresent:       boolPointer(true),
	})
	if got := snapshot.Features[2]; got.State != systemInfoOn || !strings.Contains(got.Detail, "hypervisor is running") {
		t.Fatalf("virtualization feature = %#v", got)
	}
}
