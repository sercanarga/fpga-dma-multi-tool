//go:build !windows

package main

import (
	"context"
	"errors"
)

func inspectFPGAAdvancedInfo(
	context.Context,
	deviceResult,
) (fpgaAdvancedSnapshot, error) {
	return fpgaAdvancedSnapshot{}, errors.New(
		"live FPGA register details are available in the Windows build",
	)
}

func inspectFPGAXADC(
	context.Context,
	deviceResult,
) (fpgaXADCInfo, string, error) {
	return fpgaXADCInfo{}, "", errors.New(
		"live FPGA sensor details are available in the Windows build",
	)
}

func inspectFPGAFlash(
	context.Context,
	deviceResult,
) (fpgaFlashInfo, string, error) {
	return fpgaFlashInfo{}, "", errors.New(
		"live FPGA flash details are available in the Windows build",
	)
}
