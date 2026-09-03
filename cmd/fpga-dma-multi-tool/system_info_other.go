//go:build !windows

package main

import (
	"context"
)

func inspectSystemInfo(context.Context) (systemInfoSnapshot, error) {
	return systemInfoSnapshot{
		ProcessorName:   "Windows system information is unavailable on this platform.",
		SystemSummary:   "Unavailable on this platform",
		FirmwareSummary: "Unavailable on this platform",
		WindowsSummary:  "Unavailable on this platform",
		Features: []systemInfoFeature{
			{Name: "Firmware / BIOS mode", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Secure Boot", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Hardware virtualization", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Windows hypervisor", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Kernel DMA protection", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Virtualization-based security", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Core isolation / Memory integrity", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Credential Guard", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "System Guard Secure Launch", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Trusted Platform Module", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
		},
	}, nil
}
