//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

func configureChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}

func validateSpeedTestEnvironment() error {
	executable, err := speedTestPath()
	if err != nil {
		return err
	}
	dllPath := filepath.Join(filepath.Dir(executable), "FTD3XX.dll")
	connected, err := inspectFTDID3XXAdapter(dllPath)
	if err != nil {
		return err
	}
	if !connected {
		return errors.New("FT600/FT601 adapter was not found; connect the board's USB 3 cable and try again")
	}
	return nil
}

func inspectFTDID3XXAdapter(dllPath string) (bool, error) {
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return false, fmt.Errorf("FTDI D3XX runtime could not be loaded: %w", err)
	}
	defer dll.Release()
	createDeviceInfoList, err := dll.FindProc("FT_CreateDeviceInfoList")
	if err != nil {
		return false, fmt.Errorf("FTDI D3XX runtime is incomplete: %w", err)
	}
	var deviceCount uint32
	status, _, _ := createDeviceInfoList.Call(uintptr(unsafe.Pointer(&deviceCount)))
	runtime.KeepAlive(&deviceCount)
	if status != 0 {
		return false, fmt.Errorf("FTDI D3XX device scan failed with status %d", status)
	}
	return deviceCount > 0, nil
}
