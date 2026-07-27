# Architecture

The application intentionally remains one Go `main` package so the Fyne Windows
build stays simple. Source files are grouped by feature instead of by technical
layer.

## Source layout

- `main.go` contains the CLI entry point and command-line routing.
- `board_catalog.go` maps supported Artix-7 IDCODEs and board families.
- `jtag_*` contains CH347 and XVC JTAG transports.
- `firmware_*` validates firmware requests and runs openFPGALoader.
- `dma_benchmark_*` runs and parses the DMA performance benchmark.
- `device_history_*` inspects and removes disconnected Windows Plug and Play
  entries across hardware and software enumerators.
- `driver_*` detects, installs, and removes Windows driver packages.
- `file_browser.go` contains filesystem/path logic without UI dependencies.
- `ui_app.go` creates the application and tabs.
- `ui_components.go` contains shared Windows-style controls and dialogs.
- `ui_devices.go`, `ui_flash.go`, `ui_speed.go`, `ui_history.go`,
  `ui_system_info.go`, and `ui_drivers.go` each own one application tab.
- `system_info_*` reads processor virtualization, Device Guard, Secure Boot,
  and per-device PCIe link-width state from Windows.
- `ui_file_browser.go` renders the custom Explorer-style firmware browser.
- `ui_theme.go` and `ui_font_*` contain the Windows visual theme.

Platform-specific implementations use Go build suffixes:

- `_windows.go` for the Windows implementation.
- `_other.go` for the non-Windows diagnostic stub used by development builds.

Tests live beside the feature they exercise and use the same base filename.

## Runtime flow

The Flash tab produces a validated firmware request and selects a writer:

1. `Auto` attempts CH347 first.
2. If CH347 is unavailable, it attempts a supported RS232 writer.
3. CH347 runs through the local XVC bridge.
4. RS232 uses FTDI Interface A through the FTDI-enabled openFPGALoader runtime.
5. RS DMA writers try the `0x98/0x8B` MPSSE layout used by the public RS DMA
   tools before falling back to standard Digilent/FTDI layouts.

The Drivers tab owns driver detection and setup. It does not silently replace a
generic FTDI device: RS232 setup is limited to the documented writer USB IDs and
the user confirms the exact device in Zadig. Detection and removal support
WinUSB, libusbK, and libusb0 packages bound to Interface 0 or to the composite
device node.

The System Info tab reports firmware-enabled CPU virtualization, Windows
DMA-remapping and Memory Integrity state, UEFI Secure Boot, and negotiated PCIe
link widths. PCIe rows are omitted when a device or its PnP driver does not
publish the standard link-width properties.
