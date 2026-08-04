package main

import (
	"strings"
	"testing"
)

func TestParseOpenFPGALoaderRegister(t *testing.T) {
	output := `
Jtag frequency : requested 6.00MHz -> real 6.00MHz
Register raw value: 0x14007
CRC Error       No CRC error
`
	value, err := parseOpenFPGALoaderRegister(output)
	if err != nil {
		t.Fatal(err)
	}
	if value != 0x14007 {
		t.Fatalf("register = 0x%08X, want 0x00014007", value)
	}
}

func TestFPGAConfigurationDetailRowsDecodeStatusAndBootErrors(t *testing.T) {
	registers := map[string]uint32{
		"STAT":    (1 << 14) | (1 << 4) | (1 << 12) | (1 << 15),
		"BOOTSTS": 1 | (1 << 5),
		"WBSTAR":  0x00012345,
		"CTRL0":   (1 << 0) | (1 << 3) | (1 << 10),
	}
	rows := fpgaConfigurationDetailRows(registers)
	report := detailRowsText(rows)
	for _, want := range []string{
		"Configuration state=Configured",
		"IDCODE error=Detected",
		"Status 0=CRC error",
		"Start address=0x00012345",
		"PERSIST=Enabled",
		"Configuration fallback=Enabled",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report does not contain %q:\n%s", want, report)
		}
	}
}

func TestFPGADeviceDetailRowsIncludeIdentityAndLoadingState(t *testing.T) {
	device := deviceResult{
		Index: 1, Part: "xc7a100t", IDCode: "0x03631093",
		FuseDNA: "0x0123456789ABCDEF", DeviceDNA: "0x002468ACF13579",
		BoardMatches: []string{"CaptainDMA_100T"}, Backend: "ch347-direct",
		Target: "CH347[0]",
	}
	report := detailRowsText(fpgaDeviceDetailRows(device, nil))
	for _, want := range []string{
		"FPGA=XC7A100T",
		"Possible boards=CaptainDMA_100T",
		"IR length=6 bits",
		"Registers=Reading…",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report does not contain %q:\n%s", want, report)
		}
	}
}

func TestParseOpenFPGALoaderXADC(t *testing.T) {
	output := `Jtag frequency: 6000000
{
"temp": 44.25,
"maxtemp": 61.5,
"mintemp": 32.75,
"raw": {"0": 41234, "1": 21840, "2": 39312, "6": 22016},
"vccint": 1.001,
"maxvccint": 1.018,
"minvccint": 0.988,
"vccaux": 1.803,
"maxvccaux": 1.821,
"minvccaux": 1.786
}`
	sensors, err := parseOpenFPGALoaderXADC(output)
	if err != nil {
		t.Fatal(err)
	}
	if sensors.Temperature != 44.25 || sensors.VCCINT != 1.001 {
		t.Fatalf("sensors = %#v", sensors)
	}
	if sensors.VCCBRAM < 1.007 || sensors.VCCBRAM > 1.009 {
		t.Fatalf("VCCBRAM = %.6f", sensors.VCCBRAM)
	}
}

func TestParseOpenFPGALoaderFlash(t *testing.T) {
	output := `
Detect flash:
JEDEC ID: 0x20ba18
Detected: Micron N25Q128_3V 256 sectors size: 128Mb
RDSR : 0x40
WIP  : 0
WEL  : 0
BP   : 0
TB   : 0
QE   : 1
Done
`
	flash, err := parseOpenFPGALoaderFlash(output)
	if err != nil {
		t.Fatal(err)
	}
	if flash.JEDECID != "0x20BA18" ||
		flash.Manufacturer != "Micron" ||
		flash.Model != "N25Q128_3V" ||
		flash.CapacityMbit != 128 ||
		flash.Status != "0x40" ||
		flash.QuadEnabled != "Enabled" {
		t.Fatalf("flash = %#v", flash)
	}
}

func detailRowsText(rows []fpgaDetailRow) string {
	var values []string
	for _, row := range rows {
		values = append(values, row.Field+"="+row.Value)
	}
	return strings.Join(values, "\n")
}
