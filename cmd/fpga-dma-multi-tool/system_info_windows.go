//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func inspectSystemInfo(ctx context.Context) (systemInfoSnapshot, error) {
	script := `
$cpu = Get-CimInstance Win32_Processor -ErrorAction Stop | Select-Object -First 1

function Convert-NullableBoolean($value) {
    if ($null -eq $value) {
        return $null
    }
    return [bool]$value
}

$deviceGuard = Get-CimInstance -Namespace 'root\Microsoft\Windows\DeviceGuard' -ClassName Win32_DeviceGuard -ErrorAction SilentlyContinue
$deviceGuardAvailable = $null -ne $deviceGuard
$iommuAvailable = $null
$coreIsolationConfigured = $null
$coreIsolationRunning = $null
if ($deviceGuardAvailable) {
    $iommuAvailable = [bool](@($deviceGuard.AvailableSecurityProperties) -contains 3)
    $coreIsolationConfigured = [bool](@($deviceGuard.SecurityServicesConfigured) -contains 2)
    $coreIsolationRunning = [bool](@($deviceGuard.SecurityServicesRunning) -contains 2)
}

$secureBootSupported = $false
$secureBootEnabled = $null
try {
    $secureBootEnabled = [bool](Confirm-SecureBootUEFI -ErrorAction Stop)
    $secureBootSupported = $true
} catch {
    $secureBootState = Get-ItemProperty -LiteralPath 'HKLM:\SYSTEM\CurrentControlSet\Control\SecureBoot\State' -Name UEFISecureBootEnabled -ErrorAction SilentlyContinue
    if ($null -ne $secureBootState) {
        $secureBootEnabled = [bool]$secureBootState.UEFISecureBootEnabled
        $secureBootSupported = $true
    }
}

$pcieLinks = @()
$pciDevices = @(Get-PnpDevice -PresentOnly -Status OK -ErrorAction SilentlyContinue |
    Where-Object { $_.InstanceId -like 'PCI\*' })
$linkWidths = @{}
if ($pciDevices.Count -gt 0) {
    $propertyKeys = @(
        'DEVPKEY_PciDevice_CurrentLinkWidth',
        'DEVPKEY_PciDevice_MaxLinkWidth'
    )
    $properties = @(Get-PnpDeviceProperty -InstanceId @($pciDevices.InstanceId) -KeyName $propertyKeys -ThrottleLimit 4 -ErrorAction SilentlyContinue)
    foreach ($property in $properties) {
        $deviceID = [string]$property.InstanceId
        if ([string]::IsNullOrWhiteSpace($deviceID)) {
            $deviceID = [string]$property.DeviceID
        }
        if ([string]::IsNullOrWhiteSpace($deviceID)) {
            continue
        }
        if (-not $linkWidths.ContainsKey($deviceID)) {
            $linkWidths[$deviceID] = @{
                CurrentWidth = 0
                MaximumWidth = 0
            }
        }
        if ($property.KeyName -eq 'DEVPKEY_PciDevice_CurrentLinkWidth' -and $null -ne $property.Data) {
            $linkWidths[$deviceID].CurrentWidth = [int]$property.Data
        }
        if ($property.KeyName -eq 'DEVPKEY_PciDevice_MaxLinkWidth' -and $null -ne $property.Data) {
            $linkWidths[$deviceID].MaximumWidth = [int]$property.Data
        }
    }
}
foreach ($device in $pciDevices) {
    $width = $linkWidths[[string]$device.InstanceId]
    $currentWidth = [int]$width.CurrentWidth
    $maximumWidth = [int]$width.MaximumWidth
    if ($currentWidth -gt 0) {
        $name = [string]$device.FriendlyName
        if ([string]::IsNullOrWhiteSpace($name)) {
            $name = [string]$device.InstanceId
        }
        $pcieLinks += [pscustomobject]@{
            Name = $name
            InstanceID = [string]$device.InstanceId
            CurrentWidth = $currentWidth
            MaximumWidth = $maximumWidth
        }
    }
}
$pcieLinks = @($pcieLinks | Sort-Object CurrentWidth, Name -Descending)

[pscustomobject]@{
    processorName = [string]$cpu.Name
    processorManufacturer = [string]$cpu.Manufacturer
    virtualizationSupported = Convert-NullableBoolean $cpu.VMMonitorModeExtensions
    virtualizationEnabled = Convert-NullableBoolean $cpu.VirtualizationFirmwareEnabled
    iommuAvailable = $iommuAvailable
    secureBootSupported = $secureBootSupported
    secureBootEnabled = $secureBootEnabled
    coreIsolationAvailable = $deviceGuardAvailable
    coreIsolationConfigured = $coreIsolationConfigured
    coreIsolationRunning = $coreIsolationRunning
    pcieLinks = @($pcieLinks)
} | ConvertTo-Json -Depth 5 -Compress
`
	command := exec.CommandContext(
		ctx,
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		script,
	)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return systemInfoSnapshot{}, fmt.Errorf(
			"could not inspect Windows system information: %w\n%s",
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return decodeSystemInfoJSON(output)
}
