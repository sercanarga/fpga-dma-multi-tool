package main

import "testing"

func boolPointer(value bool) *bool {
	return &value
}

func TestDecodeSystemInfoJSONBuildsAMDFeatureNamesAndStates(t *testing.T) {
	encoded := []byte(`{
		"processorName":"AMD Ryzen Test",
		"processorManufacturer":"AuthenticAMD",
		"virtualizationSupported":true,
		"virtualizationEnabled":true,
		"iommuAvailable":false,
		"secureBootSupported":true,
		"secureBootEnabled":true,
		"coreIsolationAvailable":true,
		"coreIsolationConfigured":true,
		"coreIsolationRunning":true,
		"pcieLinks":[{"Name":"DMA adapter","InstanceID":"PCI\\VEN_1234","CurrentWidth":4,"MaximumWidth":4}]
	}`)
	snapshot, err := decodeSystemInfoJSON(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProcessorName != "AMD Ryzen Test" {
		t.Fatalf("processor = %q", snapshot.ProcessorName)
	}
	if got := snapshot.Features[0]; got.Name != "AMD-V / SVM" || got.State != systemInfoOn {
		t.Fatalf("virtualization feature = %#v", got)
	}
	if got := snapshot.Features[1]; got.Name != "AMD-Vi / IOMMU" || got.State != systemInfoOff {
		t.Fatalf("IOMMU feature = %#v", got)
	}
	if got := snapshot.Features[2]; got.State != systemInfoOn {
		t.Fatalf("Secure Boot feature = %#v", got)
	}
	if got := snapshot.Features[3]; got.State != systemInfoOn {
		t.Fatalf("Core isolation feature = %#v", got)
	}
	if len(snapshot.PCIeLinks) != 1 || snapshot.PCIeLinks[0].CurrentWidth != 4 {
		t.Fatalf("PCIe links = %#v", snapshot.PCIeLinks)
	}
}

func TestBuildSystemInfoSnapshotReportsDisabledIntelFeatures(t *testing.T) {
	snapshot := buildSystemInfoSnapshot(rawSystemInfo{
		ProcessorManufacturer:   "GenuineIntel",
		VirtualizationSupported: boolPointer(true),
		VirtualizationEnabled:   boolPointer(false),
		IOMMUAvailable:          boolPointer(false),
		SecureBootSupported:     true,
		SecureBootEnabled:       boolPointer(false),
		CoreIsolationAvailable:  true,
		CoreIsolationConfigured: boolPointer(true),
		CoreIsolationRunning:    boolPointer(false),
	})
	wantNames := []string{
		"Intel VT-x",
		"Intel VT-d / IOMMU",
		"Secure Boot",
		"Core isolation / Memory integrity",
	}
	for index, want := range wantNames {
		if snapshot.Features[index].Name != want {
			t.Fatalf("feature %d name = %q, want %q", index, snapshot.Features[index].Name, want)
		}
		if snapshot.Features[index].State != systemInfoOff {
			t.Fatalf("feature %d state = %q, want Off", index, snapshot.Features[index].State)
		}
	}
}
