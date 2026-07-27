//go:build !windows

package main

import (
	"context"
)

func inspectSystemInfo(context.Context) (systemInfoSnapshot, error) {
	return systemInfoSnapshot{
		ProcessorName: "Windows system information is unavailable on this platform.",
		Features: []systemInfoFeature{
			{Name: "Hardware virtualization", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "IOMMU / DMA remapping", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Secure Boot", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
			{Name: "Core isolation / Memory integrity", State: systemInfoUnavailable, Detail: "Available in the Windows build."},
		},
	}, nil
}
