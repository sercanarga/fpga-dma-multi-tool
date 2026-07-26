//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func detectRS232Devices(ctx context.Context) ([]rs232Device, error) {
	script := `
$devicePattern = [regex]::new(
    '^USB\\VID_(0403)&PID_(6010|6011|6014)(?:&MI_(00))?\\',
    [Text.RegularExpressions.RegexOptions]::IgnoreCase
)
$signedDrivers = @(Get-CimInstance Win32_PnPSignedDriver -ErrorAction SilentlyContinue)
$devices = Get-CimInstance Win32_PnPEntity | Where-Object {
    $devicePattern.IsMatch([string]$_.PNPDeviceID)
}
foreach ($device in $devices) {
    $identity = $devicePattern.Match([string]$device.PNPDeviceID)
    if ($identity.Success) {
        function Get-DevicePropertyValue([string]$keyName) {
            $parameters = @{
                InstanceId = $device.PNPDeviceID
                KeyName = $keyName
                ErrorAction = 'SilentlyContinue'
            }
            return (Get-PnpDeviceProperty @parameters).Data
        }
        $service = [string](Get-DevicePropertyValue 'DEVPKEY_Device_Service')
        if ([string]::IsNullOrWhiteSpace($service)) {
            $service = [string]$device.Service
        }
        $inf = [string](Get-DevicePropertyValue 'DEVPKEY_Device_DriverInfPath')
        if ([string]::IsNullOrWhiteSpace($inf)) {
            $signed = $signedDrivers | Where-Object {
                [string]::Equals(
                    [string]$_.DeviceID,
                    [string]$device.PNPDeviceID,
                    [StringComparison]::OrdinalIgnoreCase
                )
            } | Select-Object -First 1
            $inf = [string]$signed.InfName
        }
        $serial = ([string]$device.PNPDeviceID -split '\\')[-1]
        $interface = if ($identity.Groups[3].Success) {
            $identity.Groups[3].Value.ToUpperInvariant()
        } else {
            "device"
        }
        @(
            $identity.Groups[1].Value.ToUpperInvariant(),
            $identity.Groups[2].Value.ToUpperInvariant(),
            $interface,
            $service,
            ([string]$device.Name -replace [char]9, " "),
            ([string]$device.Manufacturer -replace [char]9, " "),
            [IO.Path]::GetFileName([string]$inf),
            [string]$device.PNPDeviceID,
            [string]$serial
        ) -join [char]9
    }
}
`
	command := exec.CommandContext(
		ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-Command", script,
	)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"could not inspect connected RS232 writers: %w\n%s",
			err, strings.TrimSpace(string(output)),
		)
	}
	var devices []rs232Device
	for _, line := range strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 9)
		if len(fields) != 9 {
			continue
		}
		devices = append(devices, rs232Device{
			VID:          strings.ToUpper(strings.TrimSpace(fields[0])),
			PID:          strings.ToUpper(strings.TrimSpace(fields[1])),
			Interface:    strings.TrimSpace(fields[2]),
			Service:      strings.TrimSpace(fields[3]),
			Name:         strings.TrimSpace(fields[4]),
			Manufacturer: strings.TrimSpace(fields[5]),
			DriverINF:    strings.ToLower(strings.TrimSpace(fields[6])),
			InstanceID:   strings.TrimSpace(fields[7]),
			Serial:       strings.TrimSpace(fields[8]),
		})
	}
	return devices, nil
}

func inspectRS232Driver(ctx context.Context) (bool, string) {
	devices, err := detectRS232Devices(ctx)
	if err == nil && len(devices) > 0 {
		for _, device := range devices {
			if rs232ServiceReady(device.Service) {
				name := strings.TrimSpace(device.Name)
				if name == "" {
					name = "FTDI RS232 writer"
				}
				return true, fmt.Sprintf(
					"Driver and writer are ready: %s (%s:%s, Interface A, %s).",
					name, device.VID, device.PID, device.Service,
				)
			}
		}
		return false,
			"RS232 writer detected, but Interface A is not using WinUSB. Click Install and select the exact Interface 0 device."
	}
	if rs232DriverPackageInstalled() {
		return true, "WinUSB package is installed. Connect the RS232 writer to scan it."
	}
	if err != nil {
		return false, "Could not inspect connected RS232 writers."
	}
	return false, "Connect the RS232 writer, then install WinUSB for Interface 0."
}

func rs232DriverPackageInstalled() bool {
	windowsDirectory := strings.TrimSpace(os.Getenv("WINDIR"))
	if windowsDirectory == "" {
		return false
	}
	publishedINFs, _ := filepath.Glob(filepath.Join(windowsDirectory, "INF", "oem*.inf"))
	for _, publishedINF := range publishedINFs {
		contents, err := os.ReadFile(publishedINF)
		if err == nil && rs232DriverINFMatches(contents) {
			return true
		}
	}
	return false
}

func installRS232Driver(ctx context.Context) error {
	devices, err := detectRS232Devices(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return errors.New(
			"connect the FTDI writer first; supported USB IDs are 0403:6010/6011/6014 on Interface 0 or the device node",
		)
	}
	for _, device := range devices {
		if rs232ServiceReady(device.Service) {
			return nil
		}
	}
	zadig, err := bundledPath("drivers", "rs232", "zadig-2.9.exe")
	if err != nil {
		return err
	}
	verifyScript := `
$signature = Get-AuthenticodeSignature -LiteralPath $env:FPGA_DMA_MULTI_TOOL_ZADIG
if ($signature.Status -ne 'Valid') {
    throw "Zadig signature is not valid: $($signature.Status)"
}
if ($signature.SignerCertificate.Subject -notmatch '(?i)Akeo Consulting') {
    throw "Unexpected Zadig publisher: $($signature.SignerCertificate.Subject)"
}
`
	verify := exec.CommandContext(
		ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-Command", verifyScript,
	)
	configureChildProcess(verify)
	verify.Env = append(os.Environ(), "FPGA_DMA_MULTI_TOOL_ZADIG="+zadig)
	if output, verifyErr := verify.CombinedOutput(); verifyErr != nil {
		return fmt.Errorf(
			"RS232 installer verification failed: %w\n%s",
			verifyErr, strings.TrimSpace(string(output)),
		)
	}
	command := exec.CommandContext(ctx, zadig)
	if err := command.Run(); err != nil {
		return fmt.Errorf("RS232 driver installer failed: %w", err)
	}
	updated, err := detectRS232Devices(ctx)
	if err != nil {
		return err
	}
	for _, device := range updated {
		if rs232ServiceReady(device.Service) {
			return nil
		}
	}
	return errors.New(
		"WinUSB was not installed for the writer; in Zadig select FT2232H/FT4232H/FT232H Interface 0 and choose WinUSB",
	)
}

func uninstallRS232Driver(ctx context.Context) error {
	devices, err := detectRS232Devices(ctx)
	if err != nil {
		return err
	}
	infNames := rs232DriverINFs(devices)

	script := `
$elevatedScript = @'
$candidateInfs = @($env:FPGA_DMA_MULTI_TOOL_RS232_INFS -split ';' | Where-Object {
    $_ -match '(?i)^oem[0-9]+\.inf$'
})
Get-ChildItem -LiteralPath "$env:WINDIR\INF" -Filter 'oem*.inf' | ForEach-Object {
    $text = Get-Content -LiteralPath $_.FullName -Raw -ErrorAction SilentlyContinue
    if (
        [regex]::IsMatch(
            $text,
            '(?i)(VID_0403&PID_6010(?:(?:&MI_00)|(?!&MI_))|VID_0403&PID_6011(?:(?:&MI_00)|(?!&MI_))|VID_0403&PID_6014(?:(?:&MI_00)|(?!&MI_)))'
        ) -and
        [regex]::IsMatch($text, '(?i)(WinUSB|libusbK|libusb0)')
    ) {
        $candidateInfs += $_.Name
    }
}
$candidateInfs = @($candidateInfs | Sort-Object -Unique)
if ($candidateInfs.Count -eq 0) {
    exit 3
}
foreach ($inf in $candidateInfs) {
    & pnputil.exe /delete-driver $inf /uninstall /force
    if ($LASTEXITCODE -notin @(0, 259, 1641, 3010)) {
        exit $LASTEXITCODE
    }
}
exit 0
'@
$encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($elevatedScript))
$process = Start-Process -FilePath 'powershell.exe' -ArgumentList @(
    '-NoLogo', '-NoProfile', '-NonInteractive', '-EncodedCommand', $encoded
) -Verb RunAs -WindowStyle Hidden -Wait -PassThru
if ($process.ExitCode -eq 3) {
    throw "RS232 WinUSB driver package is not installed"
}
if ($process.ExitCode -notin @(0, 259, 1641, 3010)) {
    throw "pnputil exited with code $($process.ExitCode)"
}
`
	command := exec.CommandContext(
		ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-Command", script,
	)
	configureChildProcess(command)
	command.Env = append(
		os.Environ(),
		"FPGA_DMA_MULTI_TOOL_RS232_INFS="+strings.Join(infNames, ";"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"RS232 WinUSB driver removal failed: %w\n%s",
			err, strings.TrimSpace(string(output)),
		)
	}
	updated, err := detectRS232Devices(ctx)
	if err != nil {
		return fmt.Errorf("RS232 driver was removed, but verification failed: %w", err)
	}
	for _, device := range updated {
		if rs232ServiceReady(device.Service) {
			return fmt.Errorf(
				"Windows still reports %s on %s; unplug the writer, reconnect it, and try Remove again",
				device.Service,
				device.InstanceID,
			)
		}
	}
	return nil
}
