//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func inspectSystemComponents(ch347DLL, _ string) []componentStatus {
	wchInstalled, wchDriverDetails := inspectDriverPackage(
		"ch341wdm.inf",
		"WCH CH341/CH347 driver is installed.",
		"WCH CH341/CH347 driver is not installed.",
	)
	library, _, dllErr := loadCH347Library(strings.TrimSpace(ch347DLL))
	adapterFound := false
	if dllErr == nil {
		for index := 0; index < ch347MaxAdapters; index++ {
			if library.openDevice(uint32(index)) {
				adapterFound = true
				library.closeDevice(uint32(index))
				break
			}
		}
		library.dll.Release()
	}
	ch347Details := wchDriverDetails
	if wchInstalled && dllErr != nil {
		ch347Details = "The driver is installed, but the bundled CH347 library could not be loaded."
	} else if wchInstalled && adapterFound {
		ch347Details = "Driver and adapter are ready."
	} else if wchInstalled {
		ch347Details = "Driver is installed. Connect the adapter to scan."
	}
	statuses := []componentStatus{{
		Name:      "CH347 driver",
		Installed: wchInstalled,
		Details:   ch347Details,
	}}

	ftdiReady, ftdiDetails := inspectFTDID3XX()
	if ftdiReady {
		dllPath, pathErr := bundledPath("FTD3XX.dll")
		if pathErr != nil {
			ftdiDetails = "Driver is installed, but the bundled D3XX runtime is missing."
		} else if ftdiAdapterFound, adapterErr := inspectFTDID3XXAdapter(dllPath); adapterErr != nil {
			ftdiDetails = "Driver is installed, but the FT600/FT601 adapter could not be checked."
		} else if ftdiAdapterFound {
			ftdiDetails = "Driver and FT600/FT601 adapter are ready."
		} else {
			ftdiDetails = "Driver is installed. Connect the board's USB 3 cable for Speed Test."
		}
	}
	statuses = append(statuses, componentStatus{
		Name:      "FTDI D3XX driver",
		Installed: ftdiReady,
		Details:   ftdiDetails,
	})
	rs232Context, cancelRS232 := context.WithTimeout(context.Background(), 15*time.Second)
	rs232Ready, rs232Details := inspectRS232Driver(rs232Context)
	cancelRS232()
	statuses = append(statuses, componentStatus{
		Name:      "RS232 writer driver",
		Installed: rs232Ready,
		Details:   rs232Details,
	})
	return statuses
}

func runBundledWCHSetup(ctx context.Context, uninstall bool) error {
	if uninstall {
		return removeDriverPackage(
			ctx,
			"CH341WDM.INF",
			"(?i)(WinChipHead|WCH|Nanjing)",
			"WCH CH341/CH347",
		)
	}
	inf, err := bundledPath("drivers", "wch", "CH341WDM.INF")
	if err != nil {
		return err
	}
	catalog := filepath.Join(filepath.Dir(inf), "CH341WDM.CAT")
	script := `
$inf = $env:FPGA_DMA_MULTI_TOOL_WCH_INF
$catalog = $env:FPGA_DMA_MULTI_TOOL_WCH_CATALOG
if ([string]::IsNullOrWhiteSpace($inf) -or [string]::IsNullOrWhiteSpace($catalog)) {
    throw "WCH driver paths are missing"
}
$signature = Get-AuthenticodeSignature -LiteralPath $catalog
if ($signature.Status -ne 'Valid') {
    throw "WCH driver catalog signature is not valid: $($signature.Status)"
}
$subject = $signature.SignerCertificate.Subject
if ($subject -notmatch '(?i)Microsoft Windows Hardware Compatibility Publisher') {
    throw "WCH driver catalog is not WHQL signed: $subject"
}
$infText = Get-Content -LiteralPath $inf -Raw
if ($infText -notmatch '(?i)WinChipHead' -or $infText -notmatch '2\.6\.2025\.04') {
    throw "Bundled WCH driver metadata is not recognized"
}
$process = Start-Process -FilePath 'pnputil.exe' -ArgumentList @('/add-driver', $inf, '/install') -Verb RunAs -WindowStyle Hidden -Wait -PassThru
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
		"FPGA_DMA_MULTI_TOOL_WCH_INF="+inf,
		"FPGA_DMA_MULTI_TOOL_WCH_CATALOG="+catalog,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WCH driver installation failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func installFTDID3XX(ctx context.Context) error {
	inf, err := bundledPath("drivers", "ftdi", "ftdibus3.inf")
	if err != nil {
		return err
	}
	catalog := filepath.Join(filepath.Dir(inf), "ftdibus3.cat")
	script := `
$inf = $env:FPGA_DMA_MULTI_TOOL_FTDI_INF
$catalog = $env:FPGA_DMA_MULTI_TOOL_FTDI_CATALOG
if ([string]::IsNullOrWhiteSpace($inf) -or [string]::IsNullOrWhiteSpace($catalog)) {
    throw "FTDI driver paths are missing"
}
$signature = Get-AuthenticodeSignature -LiteralPath $catalog
if ($signature.Status -ne 'Valid') {
    throw "FTDI driver catalog signature is not valid: $($signature.Status)"
}
$subject = $signature.SignerCertificate.Subject
if ($subject -notmatch '(?i)Microsoft Windows Hardware Compatibility Publisher') {
    throw "FTDI driver catalog is not WHQL signed: $subject"
}
$infText = Get-Content -LiteralPath $inf -Raw
if ($infText -notmatch '(?i)FTDI' -or $infText -notmatch '1\.3\.0\.10') {
    throw "Bundled FTDI driver metadata is not recognized"
}
$process = Start-Process -FilePath 'pnputil.exe' -ArgumentList @('/add-driver', $inf, '/install') -Verb RunAs -WindowStyle Hidden -Wait -PassThru
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
		"FPGA_DMA_MULTI_TOOL_FTDI_INF="+inf,
		"FPGA_DMA_MULTI_TOOL_FTDI_CATALOG="+catalog,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("FTDI D3XX installation failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func uninstallFTDID3XX(ctx context.Context) error {
	return removeDriverPackage(
		ctx,
		"ftdibus3.inf",
		"(?i)(Future Technology Devices|FTDI)",
		"FTDI D3XX",
	)
}

func removeDriverPackage(
	ctx context.Context,
	originalINF string,
	providerPattern string,
	label string,
) error {
	script := `
$elevatedScript = @'
$originalINF = '__ORIGINAL_INF__'
$providerPattern = '__PROVIDER_PATTERN__'
$drivers = Get-WindowsDriver -Online | Where-Object {
    [IO.Path]::GetFileName($_.OriginalFileName) -ieq $originalINF -and
    $_.ProviderName -match $providerPattern
}
if (-not $drivers) {
    exit 3
}
foreach ($driver in $drivers) {
    & pnputil.exe /delete-driver $driver.Driver /uninstall
    if ($LASTEXITCODE -notin @(0, 259, 1641, 3010)) {
        exit $LASTEXITCODE
    }
}
exit 0
'@
$elevatedScript = $elevatedScript.Replace(
    '__ORIGINAL_INF__',
    $env:FPGA_DMA_MULTI_TOOL_REMOVE_INF
).Replace(
    '__PROVIDER_PATTERN__',
    $env:FPGA_DMA_MULTI_TOOL_REMOVE_PROVIDER
)
$encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($elevatedScript))
$process = Start-Process -FilePath 'powershell.exe' -ArgumentList @(
    '-NoLogo', '-NoProfile', '-NonInteractive', '-EncodedCommand', $encoded
) -Verb RunAs -WindowStyle Hidden -Wait -PassThru
if ($process.ExitCode -eq 3) {
    throw "$($env:FPGA_DMA_MULTI_TOOL_REMOVE_LABEL) driver package is not installed"
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
		"FPGA_DMA_MULTI_TOOL_REMOVE_INF="+originalINF,
		"FPGA_DMA_MULTI_TOOL_REMOVE_PROVIDER="+providerPattern,
		"FPGA_DMA_MULTI_TOOL_REMOVE_LABEL="+label,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s driver removal failed: %w\n%s", label, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func inspectFTDID3XX() (bool, string) {
	return inspectDriverPackage(
		"ftdibus3.inf",
		"FTDI D3XX driver is installed.",
		"Required by FT600/FT601 DMA adapters.",
	)
}

func inspectDriverPackage(originalINF, installedDetails, missingDetails string) (bool, string) {
	command := exec.Command("pnputil.exe", "/enum-drivers")
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err == nil && strings.Contains(strings.ToLower(string(output)), strings.ToLower(originalINF)) {
		return true, installedDetails
	}
	windowsDirectory := os.Getenv("WINDIR")
	publishedINFs, _ := filepath.Glob(filepath.Join(windowsDirectory, "INF", "oem*.inf"))
	for _, publishedINF := range publishedINFs {
		contents, readErr := os.ReadFile(publishedINF)
		if readErr == nil && driverINFMatches(contents, originalINF) {
			return true, installedDetails
		}
	}
	if err != nil {
		return false, "Could not inspect the Windows driver store."
	}
	return false, missingDetails
}
