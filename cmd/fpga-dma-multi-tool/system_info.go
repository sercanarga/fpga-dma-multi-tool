package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type systemInfoState string

const (
	systemInfoOn          systemInfoState = "On"
	systemInfoOff         systemInfoState = "Off"
	systemInfoUnavailable systemInfoState = "Unavailable"
)

type systemInfoFeature struct {
	Name   string
	State  systemInfoState
	Detail string
}

type pcieLinkInfo struct {
	Name         string
	InstanceID   string
	CurrentWidth int
	MaximumWidth int
}

type systemInfoSnapshot struct {
	ProcessorName string
	Features      []systemInfoFeature
	PCIeLinks     []pcieLinkInfo
}

type rawSystemInfo struct {
	ProcessorName           string         `json:"processorName"`
	ProcessorManufacturer   string         `json:"processorManufacturer"`
	VirtualizationSupported *bool          `json:"virtualizationSupported"`
	VirtualizationEnabled   *bool          `json:"virtualizationEnabled"`
	IOMMUAvailable          *bool          `json:"iommuAvailable"`
	SecureBootSupported     bool           `json:"secureBootSupported"`
	SecureBootEnabled       *bool          `json:"secureBootEnabled"`
	CoreIsolationAvailable  bool           `json:"coreIsolationAvailable"`
	CoreIsolationConfigured *bool          `json:"coreIsolationConfigured"`
	CoreIsolationRunning    *bool          `json:"coreIsolationRunning"`
	PCIeLinks               []pcieLinkInfo `json:"pcieLinks"`
}

func decodeSystemInfoJSON(encoded []byte) (systemInfoSnapshot, error) {
	var raw rawSystemInfo
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return systemInfoSnapshot{}, fmt.Errorf("decode Windows system information: %w", err)
	}
	return buildSystemInfoSnapshot(raw), nil
}

func buildSystemInfoSnapshot(raw rawSystemInfo) systemInfoSnapshot {
	virtualizationName := "Hardware virtualization"
	iommuName := "IOMMU / DMA remapping"
	manufacturer := strings.ToLower(raw.ProcessorManufacturer)
	switch {
	case strings.Contains(manufacturer, "intel"):
		virtualizationName = "Intel VT-x"
		iommuName = "Intel VT-d / IOMMU"
	case strings.Contains(manufacturer, "amd"):
		virtualizationName = "AMD-V / SVM"
		iommuName = "AMD-Vi / IOMMU"
	}

	virtualization := systemInfoFeature{
		Name:   virtualizationName,
		State:  systemInfoUnavailable,
		Detail: "Windows did not report the processor virtualization state.",
	}
	switch {
	case raw.VirtualizationSupported != nil && !*raw.VirtualizationSupported:
		virtualization.State = systemInfoOff
		virtualization.Detail = "The processor does not report virtual-machine monitor extensions."
	case raw.VirtualizationEnabled != nil && *raw.VirtualizationEnabled:
		virtualization.State = systemInfoOn
		virtualization.Detail = "Virtualization extensions are enabled in UEFI/BIOS."
	case raw.VirtualizationEnabled != nil:
		virtualization.State = systemInfoOff
		virtualization.Detail = "Supported by the processor, but disabled in UEFI/BIOS."
	}

	iommu := systemInfoFeature{
		Name:   iommuName,
		State:  systemInfoUnavailable,
		Detail: "Windows Device Guard did not report DMA-remapping capability.",
	}
	if raw.IOMMUAvailable != nil {
		if *raw.IOMMUAvailable {
			iommu.State = systemInfoOn
			iommu.Detail = "DMA protection and IOMMU support are available to Windows."
		} else {
			iommu.State = systemInfoOff
			iommu.Detail = "DMA protection and IOMMU support are not available to Windows."
		}
	}

	secureBoot := systemInfoFeature{
		Name:   "Secure Boot",
		State:  systemInfoUnavailable,
		Detail: "Secure Boot is unsupported or the system is using legacy BIOS mode.",
	}
	if raw.SecureBootSupported && raw.SecureBootEnabled != nil {
		if *raw.SecureBootEnabled {
			secureBoot.State = systemInfoOn
			secureBoot.Detail = "UEFI Secure Boot is enabled."
		} else {
			secureBoot.State = systemInfoOff
			secureBoot.Detail = "UEFI Secure Boot is supported but disabled."
		}
	}

	coreIsolation := systemInfoFeature{
		Name:   "Core isolation / Memory integrity",
		State:  systemInfoUnavailable,
		Detail: "Windows Device Guard runtime information is unavailable.",
	}
	if raw.CoreIsolationAvailable && raw.CoreIsolationRunning != nil {
		if *raw.CoreIsolationRunning {
			coreIsolation.State = systemInfoOn
			coreIsolation.Detail = "Hypervisor-protected code integrity is running."
		} else {
			coreIsolation.State = systemInfoOff
			if raw.CoreIsolationConfigured != nil && *raw.CoreIsolationConfigured {
				coreIsolation.Detail = "Memory integrity is configured but is not currently running."
			} else {
				coreIsolation.Detail = "Memory integrity is not running."
			}
		}
	}

	return systemInfoSnapshot{
		ProcessorName: strings.TrimSpace(raw.ProcessorName),
		Features: []systemInfoFeature{
			virtualization,
			iommu,
			secureBoot,
			coreIsolation,
		},
		PCIeLinks: append([]pcieLinkInfo(nil), raw.PCIeLinks...),
	}
}
