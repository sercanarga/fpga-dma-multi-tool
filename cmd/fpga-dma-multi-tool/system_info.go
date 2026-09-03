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
	systemInfoRunning     systemInfoState = "Running"
	systemInfoEnabled     systemInfoState = "Enabled"
	systemInfoConfigured  systemInfoState = "Configured"
	systemInfoAvailable   systemInfoState = "Available"
	systemInfoUEFI        systemInfoState = "UEFI"
	systemInfoLegacy      systemInfoState = "Legacy"
	systemInfoReady       systemInfoState = "Ready"
	systemInfoNotReady    systemInfoState = "Not ready"
	systemInfoNotPresent  systemInfoState = "Not present"
	systemInfoUnsupported systemInfoState = "Unsupported"
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
	ProcessorName   string
	SystemSummary   string
	FirmwareSummary string
	WindowsSummary  string
	Features        []systemInfoFeature
	PCIeLinks       []pcieLinkInfo
}

type rawSystemInfo struct {
	ProcessorName           string         `json:"processorName"`
	ProcessorManufacturer   string         `json:"processorManufacturer"`
	SystemManufacturer      string         `json:"systemManufacturer"`
	SystemModel             string         `json:"systemModel"`
	BIOSManufacturer        string         `json:"biosManufacturer"`
	BIOSVersion             string         `json:"biosVersion"`
	WindowsCaption          string         `json:"windowsCaption"`
	WindowsVersion          string         `json:"windowsVersion"`
	WindowsBuild            string         `json:"windowsBuild"`
	FirmwareType            *int           `json:"firmwareType"`
	VirtualizationSupported *bool          `json:"virtualizationSupported"`
	VirtualizationEnabled   *bool          `json:"virtualizationEnabled"`
	HypervisorPresent       *bool          `json:"hypervisorPresent"`
	DMAProtectionAvailable  *bool          `json:"dmaProtectionAvailable"`
	DeviceGuardAvailable    bool           `json:"deviceGuardAvailable"`
	VBSStatus               *int           `json:"vbsStatus"`
	SecureBootEnabled       *bool          `json:"secureBootEnabled"`
	MemoryIntegrityConfig   *bool          `json:"memoryIntegrityConfigured"`
	MemoryIntegrityRunning  *bool          `json:"memoryIntegrityRunning"`
	CredentialGuardConfig   *bool          `json:"credentialGuardConfigured"`
	CredentialGuardRunning  *bool          `json:"credentialGuardRunning"`
	SecureLaunchConfig      *bool          `json:"secureLaunchConfigured"`
	SecureLaunchRunning     *bool          `json:"secureLaunchRunning"`
	TPMPresent              *bool          `json:"tpmPresent"`
	TPMReady                *bool          `json:"tpmReady"`
	TPMEnabled              *bool          `json:"tpmEnabled"`
	TPMActivated            *bool          `json:"tpmActivated"`
	TPMSpecVersion          string         `json:"tpmSpecVersion"`
	PCIeLinks               []pcieLinkInfo `json:"pcieLinks"`
}

func decodeRawSystemInfoJSON(encoded []byte) (rawSystemInfo, error) {
	var raw rawSystemInfo
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return rawSystemInfo{}, fmt.Errorf("decode Windows system information: %w", err)
	}
	return raw, nil
}

func decodeSystemInfoJSON(encoded []byte) (systemInfoSnapshot, error) {
	raw, err := decodeRawSystemInfoJSON(encoded)
	if err != nil {
		return systemInfoSnapshot{}, err
	}
	return buildSystemInfoSnapshot(raw), nil
}

func systemInfoSummary(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return strings.Join(clean, " ")
}

func buildServiceFeature(name, runningDetail, configuredDetail, offDetail string, available bool, configured, running *bool) systemInfoFeature {
	feature := systemInfoFeature{Name: name, State: systemInfoUnavailable, Detail: "Windows Device Guard runtime information is unavailable."}
	if !available {
		return feature
	}
	if running != nil && *running {
		feature.State, feature.Detail = systemInfoRunning, runningDetail
	} else if configured != nil && *configured {
		feature.State, feature.Detail = systemInfoConfigured, configuredDetail
	} else if configured != nil && running != nil {
		feature.State, feature.Detail = systemInfoOff, offDetail
	}
	return feature
}

func buildSystemInfoSnapshot(raw rawSystemInfo) systemInfoSnapshot {
	virtualizationName := "Hardware virtualization"
	switch manufacturer := strings.ToLower(raw.ProcessorManufacturer); {
	case strings.Contains(manufacturer, "intel"):
		virtualizationName = "Intel VT-x"
	case strings.Contains(manufacturer, "amd"):
		virtualizationName = "AMD-V / SVM"
	}

	firmwareMode := systemInfoFeature{Name: "Firmware / BIOS mode", State: systemInfoUnavailable, Detail: "Windows did not report whether it started in UEFI or legacy BIOS mode."}
	if raw.FirmwareType != nil {
		switch *raw.FirmwareType {
		case 1:
			firmwareMode.State, firmwareMode.Detail = systemInfoLegacy, "Windows started in legacy BIOS compatibility mode."
		case 2:
			firmwareMode.State, firmwareMode.Detail = systemInfoUEFI, "Windows started in UEFI mode."
		}
	}

	secureBoot := systemInfoFeature{Name: "Secure Boot", State: systemInfoUnavailable, Detail: "Windows could not read the UEFI Secure Boot state."}
	if raw.FirmwareType != nil && *raw.FirmwareType == 1 {
		secureBoot.State, secureBoot.Detail = systemInfoUnsupported, "Secure Boot is unavailable while Windows is started in legacy BIOS mode."
	} else if raw.SecureBootEnabled != nil {
		if *raw.SecureBootEnabled {
			secureBoot.State, secureBoot.Detail = systemInfoOn, "UEFI Secure Boot is enabled."
		} else {
			secureBoot.State, secureBoot.Detail = systemInfoOff, "UEFI Secure Boot is supported but disabled."
		}
	}

	virtualization := systemInfoFeature{Name: virtualizationName, State: systemInfoUnavailable, Detail: "Windows did not report the processor virtualization state."}
	switch {
	case raw.VirtualizationEnabled != nil && *raw.VirtualizationEnabled:
		virtualization.State, virtualization.Detail = systemInfoOn, "Virtualization extensions are enabled in UEFI/BIOS."
	case raw.HypervisorPresent != nil && *raw.HypervisorPresent:
		virtualization.State, virtualization.Detail = systemInfoOn, "The Windows hypervisor is running, confirming hardware virtualization is active."
	case raw.VirtualizationSupported != nil && !*raw.VirtualizationSupported:
		virtualization.State, virtualization.Detail = systemInfoUnsupported, "The processor does not report virtual-machine monitor extensions."
	case raw.VirtualizationEnabled != nil:
		virtualization.State, virtualization.Detail = systemInfoOff, "Hardware virtualization is disabled in UEFI/BIOS."
	case raw.VirtualizationSupported != nil && *raw.VirtualizationSupported:
		virtualization.Detail = "The processor supports virtualization, but Windows did not report the firmware setting."
	}

	hypervisor := systemInfoFeature{Name: "Windows hypervisor", State: systemInfoUnavailable, Detail: "Windows did not report whether a hypervisor is present."}
	if raw.HypervisorPresent != nil {
		if *raw.HypervisorPresent {
			hypervisor.State, hypervisor.Detail = systemInfoRunning, "A Windows hypervisor is currently running."
		} else {
			hypervisor.State, hypervisor.Detail = systemInfoOff, "No Windows hypervisor is currently running."
		}
	}

	dmaProtection := systemInfoFeature{Name: "Kernel DMA protection", State: systemInfoUnavailable, Detail: "Windows did not report DMA-protection capability; the BIOS IOMMU switch cannot be inferred."}
	if raw.DMAProtectionAvailable != nil && *raw.DMAProtectionAvailable {
		dmaProtection.State, dmaProtection.Detail = systemInfoAvailable, "Windows reports DMA protection available; this is not a direct BIOS IOMMU-toggle reading."
	}

	vbs := systemInfoFeature{Name: "Virtualization-based security", State: systemInfoUnavailable, Detail: "Windows Device Guard did not report the VBS runtime state."}
	if raw.VBSStatus != nil {
		switch *raw.VBSStatus {
		case 0:
			vbs.State, vbs.Detail = systemInfoOff, "Virtualization-based security is disabled."
		case 1:
			vbs.State, vbs.Detail = systemInfoEnabled, "Virtualization-based security is enabled but is not currently running."
		case 2:
			vbs.State, vbs.Detail = systemInfoRunning, "Virtualization-based security is enabled and running."
		}
	}

	memoryIntegrity := buildServiceFeature("Core isolation / Memory integrity", "Hypervisor-protected code integrity is running.", "Memory integrity is configured but is not currently running.", "Memory integrity is not configured or running.", raw.DeviceGuardAvailable, raw.MemoryIntegrityConfig, raw.MemoryIntegrityRunning)
	credentialGuard := buildServiceFeature("Credential Guard", "Windows Defender Credential Guard is running.", "Credential Guard is configured but is not currently running.", "Credential Guard is not configured or running.", raw.DeviceGuardAvailable, raw.CredentialGuardConfig, raw.CredentialGuardRunning)
	secureLaunch := buildServiceFeature("System Guard Secure Launch", "System Guard Secure Launch is running.", "Secure Launch is configured but is not currently running.", "Secure Launch is not configured or running.", raw.DeviceGuardAvailable, raw.SecureLaunchConfig, raw.SecureLaunchRunning)

	tpm := systemInfoFeature{Name: "Trusted Platform Module", State: systemInfoUnavailable, Detail: "Windows could not read TPM status."}
	if raw.TPMPresent != nil {
		specificationDetail := ""
		if specification := strings.TrimSpace(raw.TPMSpecVersion); specification != "" {
			specificationDetail = " Specification: " + specification + "."
		}
		switch {
		case !*raw.TPMPresent:
			tpm.State, tpm.Detail = systemInfoNotPresent, "Windows did not find a compatible TPM."
		case raw.TPMReady != nil && *raw.TPMReady:
			tpm.State, tpm.Detail = systemInfoReady, "The TPM is present and ready for use."+specificationDetail
		case raw.TPMEnabled != nil && !*raw.TPMEnabled:
			tpm.State, tpm.Detail = systemInfoNotReady, "The TPM is present but reports disabled."+specificationDetail
		case raw.TPMActivated != nil && !*raw.TPMActivated:
			tpm.State, tpm.Detail = systemInfoNotReady, "The TPM is present but reports not activated."+specificationDetail
		default:
			tpm.State, tpm.Detail = systemInfoNotReady, "The TPM is present but is not ready for use."+specificationDetail
		}
	}

	firmwareSummary := systemInfoSummary(raw.BIOSManufacturer, raw.BIOSVersion)
	if firmwareSummary == "" {
		firmwareSummary = "Unavailable"
	}
	windowsSummary := systemInfoSummary(raw.WindowsCaption, raw.WindowsVersion)
	if build := strings.TrimSpace(raw.WindowsBuild); build != "" && !strings.Contains(windowsSummary, build) {
		windowsSummary = systemInfoSummary(windowsSummary, "(build "+build+")")
	}
	if windowsSummary == "" {
		windowsSummary = "Unavailable"
	}
	systemSummary := systemInfoSummary(raw.SystemManufacturer, raw.SystemModel)
	if systemSummary == "" {
		systemSummary = "Unavailable"
	}

	return systemInfoSnapshot{
		ProcessorName: strings.TrimSpace(raw.ProcessorName), SystemSummary: systemSummary, FirmwareSummary: firmwareSummary, WindowsSummary: windowsSummary,
		Features:  []systemInfoFeature{firmwareMode, secureBoot, virtualization, hypervisor, dmaProtection, vbs, memoryIntegrity, credentialGuard, secureLaunch, tpm},
		PCIeLinks: append([]pcieLinkInfo(nil), raw.PCIeLinks...),
	}
}
