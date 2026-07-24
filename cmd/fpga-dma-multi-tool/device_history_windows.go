//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type deviceHistoryEnvelope struct {
	Devices []deviceHistoryEntry `json:"Devices"`
}

func scanDeviceHistory(ctx context.Context) ([]deviceHistoryEntry, error) {
	script := `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
$present = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
Get-PnpDevice -PresentOnly -ErrorAction Stop | ForEach-Object {
    if ($_.InstanceId) { $null = $present.Add([string]$_.InstanceId) }
}
$devices = @(Get-PnpDevice -ErrorAction Stop | Where-Object {
    $_.InstanceId -match '^(PCI|USB)\\'
} | ForEach-Object {
    $segments = @([string]$_.InstanceId -split '\\', 3)
    $name = [string]$_.FriendlyName
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = [string]$_.Class
    }
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = [string]$_.InstanceId
    }
    [pscustomobject]@{
        Type = $segments[0]
        HardwareId = $segments[1]
        InstanceId = $segments[2]
        FriendlyName = $name
        Driver = ''
        ControlSets = @()
        Present = $present.Contains([string]$_.InstanceId)
    }
} | Sort-Object Type, FriendlyName, HardwareId, InstanceId)
@{ Devices = $devices } | ConvertTo-Json -Depth 4 -Compress
`
	command := exec.CommandContext(
		ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script,
	)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("device history scan failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	var envelope deviceHistoryEnvelope
	if err := json.Unmarshal(bytes.TrimSpace(output), &envelope); err != nil {
		return nil, fmt.Errorf("device history scan returned invalid data: %w", err)
	}
	return envelope.Devices, nil
}

func removeDeviceHistory(
	ctx context.Context,
	device deviceHistoryEntry,
) (deviceHistoryCleanupResult, error) {
	instanceID := deviceHistoryInstanceID(device)
	present, err := deviceInstanceIsPresent(ctx, instanceID)
	if err != nil {
		return deviceHistoryCleanupResult{}, err
	}
	if present {
		return deviceHistoryCleanupResult{}, fmt.Errorf("disconnect the device before removing its history")
	}

	command := exec.CommandContext(ctx, "pnputil.exe", "/remove-device", instanceID)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return deviceHistoryCleanupResult{}, fmt.Errorf(
			"Windows could not remove the selected device history: %w\n%s",
			err,
			strings.TrimSpace(string(output)),
		)
	}
	if err := verifyDeviceHistoryRemoved(ctx, instanceID); err != nil {
		return deviceHistoryCleanupResult{}, err
	}
	return deviceHistoryCleanupResult{}, nil
}

func deviceInstanceIsPresent(ctx context.Context, instanceID string) (bool, error) {
	return queryDeviceInstance(ctx, instanceID, true)
}

func deviceInstanceExists(ctx context.Context, instanceID string) (bool, error) {
	return queryDeviceInstance(ctx, instanceID, false)
}

func queryDeviceInstance(
	ctx context.Context,
	instanceID string,
	presentOnly bool,
) (bool, error) {
	script := `
$ProgressPreference = 'SilentlyContinue'
$parameters = @{
    InstanceId = $env:FPGA_DMA_MULTI_TOOL_DEVICE_INSTANCE
    ErrorAction = 'SilentlyContinue'
}
if ($env:FPGA_DMA_MULTI_TOOL_PRESENT_ONLY -eq '1') {
    $parameters.PresentOnly = $true
}
$device = Get-PnpDevice @parameters
if ($null -ne $device) { Write-Output 'found' } else { Write-Output 'missing' }
`
	command := exec.CommandContext(
		ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script,
	)
	configureChildProcess(command)
	presentFlag := "0"
	if presentOnly {
		presentFlag = "1"
	}
	command.Env = append(
		os.Environ(),
		"FPGA_DMA_MULTI_TOOL_DEVICE_INSTANCE="+instanceID,
		"FPGA_DMA_MULTI_TOOL_PRESENT_ONLY="+presentFlag,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("could not verify the selected device state: %w", err)
	}
	return strings.TrimSpace(string(output)) == "found", nil
}

func verifyDeviceHistoryRemoved(ctx context.Context, instanceID string) error {
	exists, err := deviceInstanceExists(ctx, instanceID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("Windows still reports the removed device instance; rescan or restart Windows and try again")
	}

	registryPath, err := deviceEnumRegistryPath(instanceID)
	if err != nil {
		return err
	}
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		registryPath,
		registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS,
	)
	if err == nil {
		key.Close()
		return fmt.Errorf("Windows removed the device node but its active registry instance is still present")
	}
	if !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("could not verify removal of the device registry instance: %w", err)
	}
	return nil
}

func rescanWindowsDevices(ctx context.Context) error {
	command := exec.CommandContext(ctx, "pnputil.exe", "/scan-devices")
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Windows device rescan failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
