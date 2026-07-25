//go:build !windows

package main

import (
	"context"
	"errors"
)

func inspectSystemComponents(string, string) []componentStatus {
	return []componentStatus{
		{Name: "CH347 driver", Installed: false, Details: "Driver setup is available in the Windows build."},
		{Name: "FTDI D3XX driver", Installed: false, Details: "Driver setup is available in the Windows build."},
		{Name: "RS232 writer driver", Installed: false, Details: "Driver setup is available in the Windows build."},
	}
}

func runBundledWCHSetup(context.Context, bool) error {
	return errors.New("WCH driver setup is available in the Windows build")
}

func installFTDID3XX(context.Context) error {
	return errors.New("FTDI D3XX setup is available in the Windows build")
}

func uninstallFTDID3XX(context.Context) error {
	return errors.New("FTDI D3XX setup is available in the Windows build")
}

func installRS232Driver(context.Context) error {
	return errors.New("RS232 driver setup is available in the Windows build")
}

func uninstallRS232Driver(context.Context) error {
	return errors.New("RS232 driver setup is available in the Windows build")
}
