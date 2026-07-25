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
$seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
$devices = [Collections.Generic.List[object]]::new()
Get-PnpDevice -ErrorAction Stop | ForEach-Object {
    if ([string]::IsNullOrWhiteSpace([string]$_.InstanceId)) {
        return
    }
    $segments = @([string]$_.InstanceId -split '\\', 3)
    if ($segments.Count -ne 3 -or
        [string]::IsNullOrWhiteSpace($segments[0]) -or
        [string]::IsNullOrWhiteSpace($segments[1]) -or
        [string]::IsNullOrWhiteSpace($segments[2]) -or
        $segments[0] -notmatch '^[A-Za-z0-9_.-]+$') {
        return
    }
    $name = [string]$_.FriendlyName
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = [string]$_.Class
    }
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = [string]$_.InstanceId
    }
    $devices.Add([pscustomobject]@{
        Enumerator = $segments[0]
        DeviceId = $segments[1]
        InstanceId = $segments[2]
        Class = [string]$_.Class
        FriendlyName = $name
        Present = $present.Contains([string]$_.InstanceId)
    })
    $null = $seen.Add([string]$_.InstanceId)
}

# Device Manager's hidden-device view is backed by the Enum registry tree.
# Merge any historical instance that Get-PnpDevice did not return so stale
# WPD, AudioEndpoint, HDAUDIO, Bluetooth and storage nodes remain visible.
$enumRoot = 'Registry::HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Enum'
Get-ChildItem -LiteralPath $enumRoot -ErrorAction Stop | ForEach-Object {
    $enumerator = [string]$_.PSChildName
    if ($enumerator -notmatch '^[A-Za-z0-9_.-]+$') {
        return
    }
    Get-ChildItem -LiteralPath $_.PSPath -ErrorAction SilentlyContinue | ForEach-Object {
        $deviceId = [string]$_.PSChildName
        Get-ChildItem -LiteralPath $_.PSPath -ErrorAction SilentlyContinue | ForEach-Object {
            $instanceId = [string]$_.PSChildName
            $fullId = $enumerator + '\' + $deviceId + '\' + $instanceId
            if ($seen.Contains($fullId)) {
                return
            }
            $properties = Get-ItemProperty -LiteralPath $_.PSPath -ErrorAction SilentlyContinue
            $class = [string]$properties.Class
            $name = [string]$properties.FriendlyName
            if ([string]::IsNullOrWhiteSpace($name)) {
                $name = [string]$properties.DeviceDesc
                $name = $name -replace '^@[^;]+;', ''
            }
            if ([string]::IsNullOrWhiteSpace($name)) {
                $name = $fullId
            }
            $devices.Add([pscustomobject]@{
                Enumerator = $enumerator
                DeviceId = $deviceId
                InstanceId = $instanceId
                Class = $class
                FriendlyName = $name
                Present = $present.Contains($fullId)
            })
            $null = $seen.Add($fullId)
        }
    }
}

$sorted = @($devices | Sort-Object Class, FriendlyName, Enumerator, DeviceId, InstanceId)
@{ Devices = $sorted } | ConvertTo-Json -Depth 4 -Compress
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
) error {
	instanceID := deviceHistoryInstanceID(device)
	if _, err := deviceEnumRegistryPath(instanceID); err != nil {
		return err
	}
	present, err := deviceInstanceIsPresent(ctx, instanceID)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("disconnect the device before removing its history")
	}

	command := exec.CommandContext(ctx, "pnputil.exe", "/remove-device", instanceID)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"Windows could not remove the selected device history: %w\n%s",
			err,
			strings.TrimSpace(string(output)),
		)
	}
	if err := verifyDeviceHistoryRemoved(ctx, instanceID); err != nil {
		return err
	}
	return nil
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
