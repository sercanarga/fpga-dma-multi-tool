//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getFirmwareTypeProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetFirmwareType")

func inspectFirmwareType() *int {
	var firmwareType uint32
	success, _, _ := getFirmwareTypeProc.Call(uintptr(unsafe.Pointer(&firmwareType)))
	if success == 0 || firmwareType == 0 {
		return nil
	}
	value := int(firmwareType)
	return &value
}

func inspectSystemInfo(ctx context.Context) (systemInfoSnapshot, error) {
	script := `
$cpu = Get-CimInstance Win32_Processor -ErrorAction Stop | Select-Object -First 1
$computerSystem = Get-CimInstance Win32_ComputerSystem -ErrorAction Stop
$bios = Get-CimInstance Win32_BIOS -ErrorAction SilentlyContinue | Select-Object -First 1
$operatingSystem = Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue | Select-Object -First 1

function Convert-NullableBoolean($value) {
    if ($null -eq $value) {
        return $null
    }
    return [bool]$value
}

function Convert-NullableInteger($value) {
    if ($null -eq $value) {
        return $null
    }
    return [int]$value
}

$deviceGuard = Get-CimInstance -Namespace 'root\Microsoft\Windows\DeviceGuard' -ClassName Win32_DeviceGuard -ErrorAction SilentlyContinue
$deviceGuardAvailable = $null -ne $deviceGuard
$dmaProtectionAvailable = $null
$vbsStatus = $null
$memoryIntegrityConfigured = $null
$memoryIntegrityRunning = $null
$credentialGuardConfigured = $null
$credentialGuardRunning = $null
$secureLaunchConfigured = $null
$secureLaunchRunning = $null
if ($deviceGuardAvailable) {
    $dmaProtectionAvailable = [bool](@($deviceGuard.AvailableSecurityProperties) -contains 3)
    $vbsStatus = Convert-NullableInteger $deviceGuard.VirtualizationBasedSecurityStatus
    $memoryIntegrityConfigured = [bool](@($deviceGuard.SecurityServicesConfigured) -contains 2)
    $memoryIntegrityRunning = [bool](@($deviceGuard.SecurityServicesRunning) -contains 2)
    $credentialGuardConfigured = [bool](@($deviceGuard.SecurityServicesConfigured) -contains 1)
    $credentialGuardRunning = [bool](@($deviceGuard.SecurityServicesRunning) -contains 1)
    $secureLaunchConfigured = [bool](@($deviceGuard.SecurityServicesConfigured) -contains 3)
    $secureLaunchRunning = [bool](@($deviceGuard.SecurityServicesRunning) -contains 3)
}

$secureBootEnabled = $null
try {
    $secureBootEnabled = [bool](Confirm-SecureBootUEFI -ErrorAction Stop)
} catch {
    $secureBootState = Get-ItemProperty -LiteralPath 'HKLM:\SYSTEM\CurrentControlSet\Control\SecureBoot\State' -Name UEFISecureBootEnabled -ErrorAction SilentlyContinue
    if ($null -ne $secureBootState) {
        $secureBootEnabled = [bool]$secureBootState.UEFISecureBootEnabled
    }
}

$tpm = $null
if ($null -ne (Get-Command Get-Tpm -ErrorAction SilentlyContinue)) {
    $tpm = Get-Tpm -ErrorAction SilentlyContinue
}
$tpmWMI = Get-CimInstance -Namespace 'root\CIMV2\Security\MicrosoftTpm' -ClassName Win32_Tpm -ErrorAction SilentlyContinue | Select-Object -First 1

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
    systemManufacturer = [string]$computerSystem.Manufacturer
    systemModel = [string]$computerSystem.Model
    biosManufacturer = [string]$bios.Manufacturer
    biosVersion = [string]$bios.SMBIOSBIOSVersion
    windowsCaption = [string]$operatingSystem.Caption
    windowsVersion = [string]$operatingSystem.Version
    windowsBuild = [string]$operatingSystem.BuildNumber
    virtualizationSupported = Convert-NullableBoolean $cpu.VMMonitorModeExtensions
    virtualizationEnabled = Convert-NullableBoolean $cpu.VirtualizationFirmwareEnabled
    hypervisorPresent = Convert-NullableBoolean $computerSystem.HypervisorPresent
    dmaProtectionAvailable = $dmaProtectionAvailable
    deviceGuardAvailable = $deviceGuardAvailable
    vbsStatus = $vbsStatus
    secureBootEnabled = $secureBootEnabled
    memoryIntegrityConfigured = $memoryIntegrityConfigured
    memoryIntegrityRunning = $memoryIntegrityRunning
    credentialGuardConfigured = $credentialGuardConfigured
    credentialGuardRunning = $credentialGuardRunning
    secureLaunchConfigured = $secureLaunchConfigured
    secureLaunchRunning = $secureLaunchRunning
    tpmPresent = Convert-NullableBoolean $tpm.TpmPresent
    tpmReady = Convert-NullableBoolean $tpm.TpmReady
    tpmEnabled = Convert-NullableBoolean $tpm.TpmEnabled
    tpmActivated = Convert-NullableBoolean $tpm.TpmActivated
    tpmSpecVersion = [string]$tpmWMI.SpecVersion
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
	raw, err := decodeRawSystemInfoJSON(output)
	if err != nil {
		return systemInfoSnapshot{}, err
	}
	raw.FirmwareType = inspectFirmwareType()
	return buildSystemInfoSnapshot(raw), nil
}
