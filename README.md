# FPGA DMA Multi Tool

A compact Windows utility for detecting, configuring, and testing supported Artix-7 FPGA boards.

## Preview

<p align="center">
  <img src="assets/screenshots/devices.png" width="49%" alt="FPGA device detection">
  <img src="assets/screenshots/flash.png" width="49%" alt="FPGA flash and SRAM programming">
  <img src="assets/screenshots/speed-test.png" width="49%" alt="DMA speed test">
  <img src="assets/screenshots/device-history.png" width="49%" alt="Windows device history">
  <img src="assets/screenshots/drivers.png" width="49%" alt="Windows driver management">
</p>

## Features

- Detect supported Artix-7 devices and read their IDCODE and factory DNA ID
- Load bitstreams into SRAM or write persistent flash
- Measure memory read and read/write performance
- Review and remove disconnected Windows device-history entries
- Install and remove the bundled CH347 and FTDI D3XX drivers

## Download

Download the latest Windows package from [Releases](https://github.com/sercanarga/fpga-dma-multi-tool/releases/latest).

## Build

```sh
make build-windows
```

Windows 10 or 11 is required. The application requests administrator access when launched.
