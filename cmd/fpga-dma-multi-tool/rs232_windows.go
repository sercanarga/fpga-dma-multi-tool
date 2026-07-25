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

type rs232Device struct {
	PID        string
	Service    string
	Name       string
	InstanceID string
}

func detectRS232Devices(ctx context.Context) ([]rs232Device, error) {
	script := `
$devices = Get-CimInstance Win32_PnPEntity | Where-Object {
    $_.PNPDeviceID -match '^USB\\(VID_0403&PID_6011&MI_00|VID_0403&PID_6014(&MI_00)?)\\'
}
foreach ($device in $devices) {
    $match = [regex]::Match($device.PNPDeviceID, '(?i)PID_(6011|6014)')
    if ($match.Success) {
        @(
            $match.Groups[1].Value.ToUpperInvariant(),
            [string]$device.Service,
            ([string]$device.Name -replace [char]9, " "),
            [string]$device.PNPDeviceID
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
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			continue
		}
		devices = append(devices, rs232Device{
			PID:        strings.ToUpper(strings.TrimSpace(fields[0])),
			Service:    strings.TrimSpace(fields[1]),
			Name:       strings.TrimSpace(fields[2]),
			InstanceID: strings.TrimSpace(fields[3]),
		})
	}
	return devices, nil
}

func rs232ServiceReady(service string) bool {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "winusb", "libusbk", "libusb0":
		return true
	default:
		return false
	}
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
					"Driver and writer are ready: %s (0403:%s, Interface A).",
					name, device.PID,
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
			"connect the RS232 writer first; supported USB IDs are 0403:6011 Interface 0 and 0403:6014",
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
		"WinUSB was not installed for the RS232 writer; in Zadig select Quad RS232-HS/FT232H Interface 0 and choose WinUSB",
	)
}

func uninstallRS232Driver(ctx context.Context) error {
	script := `
$elevatedScript = @'
$matches = @()
Get-ChildItem -LiteralPath "$env:WINDIR\INF" -Filter 'oem*.inf' | ForEach-Object {
    $text = Get-Content -LiteralPath $_.FullName -Raw -ErrorAction SilentlyContinue
    if (
        $text -match '(?i)(VID_0403&PID_6011&MI_00|VID_0403&PID_6014)' -and
        $text -match '(?i)WinUSB'
    ) {
        $matches += $_.Name
    }
}
if ($matches.Count -eq 0) {
    exit 3
}
foreach ($inf in $matches) {
    & pnputil.exe /delete-driver $inf /uninstall
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
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"RS232 WinUSB driver removal failed: %w\n%s",
			err, strings.TrimSpace(string(output)),
		)
	}
	return nil
}
