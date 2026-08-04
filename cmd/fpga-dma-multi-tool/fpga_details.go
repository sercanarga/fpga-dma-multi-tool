package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var fpgaConfigurationRegisters = []string{"STAT", "CTRL0", "BOOTSTS", "WBSTAR"}

type fpgaAdvancedSnapshot struct {
	Registers          map[string]uint32
	Transport          string
	Warnings           []string
	DisruptiveReadNote string
	Sensors            *fpgaXADCInfo
	Flash              *fpgaFlashInfo
}

type fpgaXADCInfo struct {
	Temperature    float64
	MaxTemperature float64
	MinTemperature float64
	VCCINT         float64
	MaxVCCINT      float64
	MinVCCINT      float64
	VCCAUX         float64
	MaxVCCAUX      float64
	MinVCCAUX      float64
	VCCBRAM        float64
}

type fpgaFlashInfo struct {
	JEDECID      string
	Manufacturer string
	Model        string
	CapacityMbit int
	Status       string
	Protection   string
	QuadEnabled  string
}

type fpgaDetailRow struct {
	Section string
	Field   string
	Value   string
}

func fpgaDeviceDetailRows(
	device deviceResult,
	snapshot *fpgaAdvancedSnapshot,
) []fpgaDetailRow {
	board := strings.Join(device.BoardMatches, ", ")
	if board == "" {
		board = "Unknown board"
	}
	rows := []fpgaDetailRow{
		{
			Section: "Identity",
			Field:   "FPGA",
			Value:   valueOrUnavailable(strings.ToUpper(partFamily(device.Part))),
		},
		{Section: "Identity", Field: "Part", Value: valueOrUnavailable(device.Part)},
		{Section: "Identity", Field: "Possible boards", Value: board},
		{Section: "Identity", Field: "IDCODE", Value: valueOrUnavailable(device.IDCode)},
		{Section: "Identity", Field: "FUSE DNA", Value: valueOrUnavailable(device.FuseDNA)},
		{Section: "Identity", Field: "Device DNA", Value: valueOrUnavailable(device.DeviceDNA)},
		{Section: "JTAG", Field: "Chain index", Value: fmt.Sprintf("%d", device.Index)},
		{Section: "JTAG", Field: "IR length", Value: fpgaIRLength(device)},
		{Section: "JTAG", Field: "Backend", Value: valueOrUnavailable(device.Backend)},
		{Section: "JTAG", Field: "Target", Value: valueOrUnavailable(device.Target)},
	}
	if snapshot == nil {
		return append(rows, fpgaDetailRow{
			Section: "Configuration",
			Field:   "Registers",
			Value:   "Reading…",
		})
	}
	if strings.TrimSpace(snapshot.Transport) != "" &&
		!strings.EqualFold(strings.TrimSpace(snapshot.Transport), strings.TrimSpace(device.Target)) {
		rows = append(rows, fpgaDetailRow{
			Section: "JTAG", Field: "Live transport", Value: snapshot.Transport,
		})
	}
	rows = append(rows, fpgaConfigurationDetailRows(snapshot.Registers)...)
	rows = append(rows, fpgaSensorDetailRows(snapshot.Sensors)...)
	rows = append(rows, fpgaFlashDetailRows(snapshot.Flash)...)
	for _, warning := range snapshot.Warnings {
		rows = append(rows, fpgaDetailRow{
			Section: "Warning", Field: "Partial read", Value: warning,
		})
	}
	if strings.TrimSpace(snapshot.DisruptiveReadNote) != "" {
		rows = append(rows, fpgaDetailRow{
			Section: "Safety", Field: "Additional probes", Value: snapshot.DisruptiveReadNote,
		})
	}
	return rows
}

func fpgaSensorDetailRows(sensors *fpgaXADCInfo) []fpgaDetailRow {
	if sensors == nil {
		return []fpgaDetailRow{{
			Section: "Sensors",
			Field:   "XADC",
			Value:   "Not read",
		}}
	}
	return []fpgaDetailRow{
		{
			Section: "Sensors", Field: "Temperature",
			Value: fmt.Sprintf("%.2f °C", sensors.Temperature),
		},
		{
			Section: "Sensors", Field: "Recorded temperature range",
			Value: fmt.Sprintf("%.2f–%.2f °C", sensors.MinTemperature, sensors.MaxTemperature),
		},
		{
			Section: "Sensors", Field: "VCCINT",
			Value: formatVoltageRange(sensors.VCCINT, sensors.MinVCCINT, sensors.MaxVCCINT),
		},
		{
			Section: "Sensors", Field: "VCCAUX",
			Value: formatVoltageRange(sensors.VCCAUX, sensors.MinVCCAUX, sensors.MaxVCCAUX),
		},
		{
			Section: "Sensors", Field: "VCCBRAM",
			Value: fmt.Sprintf("%.3f V", sensors.VCCBRAM),
		},
	}
}

func fpgaFlashDetailRows(flash *fpgaFlashInfo) []fpgaDetailRow {
	if flash == nil {
		return []fpgaDetailRow{{
			Section: "SPI flash",
			Field:   "Device",
			Value:   "Not probed",
		}}
	}
	rows := []fpgaDetailRow{
		{Section: "SPI flash", Field: "JEDEC ID", Value: valueOrUnavailable(flash.JEDECID)},
		{Section: "SPI flash", Field: "Manufacturer", Value: valueOrUnavailable(flash.Manufacturer)},
		{Section: "SPI flash", Field: "Model", Value: valueOrUnavailable(flash.Model)},
	}
	if flash.CapacityMbit > 0 {
		rows = append(rows, fpgaDetailRow{
			Section: "SPI flash",
			Field:   "Capacity",
			Value: fmt.Sprintf(
				"%d Mbit (%d MiB)",
				flash.CapacityMbit,
				flash.CapacityMbit/8,
			),
		})
	}
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "Status register", value: flash.Status},
		{field: "Protection", value: flash.Protection},
		{field: "Quad mode", value: flash.QuadEnabled},
	} {
		if strings.TrimSpace(item.value) != "" {
			rows = append(rows, fpgaDetailRow{
				Section: "SPI flash", Field: item.field, Value: item.value,
			})
		}
	}
	return rows
}

func fpgaConfigurationDetailRows(registers map[string]uint32) []fpgaDetailRow {
	var rows []fpgaDetailRow
	if value, ok := registers["STAT"]; ok {
		configuration := "Not complete"
		if bitSet(value, 14) && bitSet(value, 4) {
			configuration = "Configured"
		}
		rows = append(rows,
			registerRawRow("STAT", value),
			fpgaDetailRow{Section: "Configuration", Field: "Configuration state", Value: configuration},
			fpgaDetailRow{Section: "Configuration", Field: "End of startup", Value: yesNo(bitSet(value, 4))},
			fpgaDetailRow{Section: "Configuration", Field: "INIT_B", Value: highLow(bitSet(value, 12))},
			fpgaDetailRow{Section: "Configuration", Field: "CRC error", Value: clearDetected(bitSet(value, 0))},
			fpgaDetailRow{Section: "Configuration", Field: "IDCODE error", Value: clearDetected(bitSet(value, 15))},
			fpgaDetailRow{Section: "Configuration", Field: "Decryption error", Value: clearDetected(bitSet(value, 16))},
			fpgaDetailRow{Section: "Configuration", Field: "XADC over-temperature", Value: clearDetected(bitSet(value, 17))},
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "Configuration bus width",
				Value:   configurationBusWidth((value >> 25) & 0x3),
			},
		)
	}
	if value, ok := registers["CTRL0"]; ok {
		rows = append(rows,
			registerRawRow("CTRL0", value),
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "User I/O state",
				Value:   map[bool]string{true: "Active", false: "3-stated"}[bitSet(value, 0)],
			},
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "PERSIST",
				Value:   enabledDisabled(bitSet(value, 3)),
			},
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "Readback security",
				Value:   readbackSecurity((value >> 4) & 0x3),
			},
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "Configuration fallback",
				Value:   enabledDisabled(bitSet(value, 10)),
			},
			fpgaDetailRow{
				Section: "Configuration",
				Field:   "Over-temperature power-down",
				Value:   enabledDisabled(bitSet(value, 12)),
			},
		)
	}
	if value, ok := registers["BOOTSTS"]; ok {
		rows = append(rows,
			registerRawRow("BOOTSTS", value),
			fpgaDetailRow{
				Section: "Boot history",
				Field:   "Status 0",
				Value:   summarizeBootRecord(value, 0),
			},
			fpgaDetailRow{
				Section: "Boot history",
				Field:   "Status 1",
				Value:   summarizeBootRecord(value, 8),
			},
		)
	}
	if value, ok := registers["WBSTAR"]; ok {
		rows = append(rows,
			registerRawRow("WBSTAR", value),
			fpgaDetailRow{
				Section: "Warm boot",
				Field:   "Start address",
				Value:   fmt.Sprintf("0x%08X", value&0x1FFFFFFF),
			},
			fpgaDetailRow{
				Section: "Warm boot",
				Field:   "RS[1:0]",
				Value:   fmt.Sprintf("%02b", (value>>30)&0x3),
			},
			fpgaDetailRow{
				Section: "Warm boot",
				Field:   "RS pins",
				Value: map[bool]string{
					true:  "Enabled",
					false: "3-stated",
				}[bitSet(value, 29)],
			},
		)
	}
	if len(registers) == 0 {
		rows = append(rows, fpgaDetailRow{
			Section: "Configuration",
			Field:   "Registers",
			Value:   "Unavailable",
		})
	}
	return rows
}

func registerRawRow(name string, value uint32) fpgaDetailRow {
	return fpgaDetailRow{
		Section: "Registers",
		Field:   name + " raw",
		Value:   fmt.Sprintf("0x%08X", value),
	}
}

func summarizeBootRecord(value uint32, offset uint) string {
	if !bitSet(value, offset) {
		return "No valid boot record"
	}
	var states []string
	if bitSet(value, offset+1) {
		states = append(states, "Fallback")
	}
	if bitSet(value, offset+2) {
		states = append(states, "IPROG")
	}
	for bit, name := range map[uint]string{
		3: "Watchdog timeout",
		4: "IDCODE error",
		5: "CRC error",
		6: "Address wrap error",
		7: "HMAC error",
	} {
		if bitSet(value, offset+bit) {
			states = append(states, name)
		}
	}
	if len(states) == 0 {
		return "Normal configuration"
	}
	sort.Strings(states)
	return strings.Join(states, ", ")
}

func parseOpenFPGALoaderRegister(output string) (uint32, error) {
	const marker = "register raw value:"
	for _, line := range strings.Split(output, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		position := strings.Index(lower, marker)
		if position < 0 {
			continue
		}
		value := strings.TrimSpace(line[position+len(marker):])
		if fields := strings.Fields(value); len(fields) > 0 {
			value = fields[0]
		}
		parsed, err := parseHexUint64(value)
		if err != nil || parsed > uint64(^uint32(0)) {
			return 0, fmt.Errorf("invalid FPGA register value %q", value)
		}
		return uint32(parsed), nil
	}
	return 0, fmt.Errorf("programmer did not return a register value")
}

func parseOpenFPGALoaderXADC(output string) (fpgaXADCInfo, error) {
	marker := strings.Index(strings.ToLower(output), `"temp"`)
	if marker < 0 {
		return fpgaXADCInfo{}, fmt.Errorf("programmer did not return XADC measurements")
	}
	start := strings.LastIndex(output[:marker], "{")
	end := strings.LastIndex(output, "}")
	if start < 0 || end <= start {
		return fpgaXADCInfo{}, fmt.Errorf("programmer returned malformed XADC output")
	}
	var encoded struct {
		Temperature    float64           `json:"temp"`
		MaxTemperature float64           `json:"maxtemp"`
		MinTemperature float64           `json:"mintemp"`
		Raw            map[string]uint32 `json:"raw"`
		VCCINT         float64           `json:"vccint"`
		MaxVCCINT      float64           `json:"maxvccint"`
		MinVCCINT      float64           `json:"minvccint"`
		VCCAUX         float64           `json:"vccaux"`
		MaxVCCAUX      float64           `json:"maxvccaux"`
		MinVCCAUX      float64           `json:"minvccaux"`
	}
	if err := json.Unmarshal([]byte(output[start:end+1]), &encoded); err != nil {
		return fpgaXADCInfo{}, fmt.Errorf("decode XADC measurements: %w", err)
	}
	vccbram := xadcVoltage(encoded.Raw["6"])
	return fpgaXADCInfo{
		Temperature:    encoded.Temperature,
		MaxTemperature: encoded.MaxTemperature,
		MinTemperature: encoded.MinTemperature,
		VCCINT:         encoded.VCCINT,
		MaxVCCINT:      encoded.MaxVCCINT,
		MinVCCINT:      encoded.MinVCCINT,
		VCCAUX:         encoded.VCCAUX,
		MaxVCCAUX:      encoded.MaxVCCAUX,
		MinVCCAUX:      encoded.MinVCCAUX,
		VCCBRAM:        vccbram,
	}, nil
}

var detectedFlashPattern = regexp.MustCompile(
	`(?i)^Detected:\s+(\S+)\s+(.+?)\s+(\d+)\s+sectors\s+size:\s+(\d+)Mb\s*$`,
)

func parseOpenFPGALoaderFlash(output string) (fpgaFlashInfo, error) {
	var flash fpgaFlashInfo
	var unknownJEDEC, memoryType, memoryCapacity string
	var blockProtection, topBottom, quadMode string
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "jedec id:"):
			flash.JEDECID = normalizeHexLabel(strings.TrimSpace(line[len("JEDEC ID:"):]))
		case detectedFlashPattern.MatchString(line):
			match := detectedFlashPattern.FindStringSubmatch(line)
			flash.Manufacturer = strings.TrimSpace(match[1])
			flash.Model = strings.TrimSpace(match[2])
			flash.CapacityMbit, _ = strconv.Atoi(match[4])
		case strings.HasPrefix(lower, "jedec id"):
			unknownJEDEC = valueAfterColon(line)
		case strings.HasPrefix(lower, "memory type"):
			memoryType = valueAfterColon(line)
		case strings.HasPrefix(lower, "memory capacity"):
			memoryCapacity = valueAfterColon(line)
		case strings.HasPrefix(lower, "rdsr"):
			flash.Status = normalizeHexLabel(valueAfterColon(line))
		case strings.HasPrefix(lower, "bp"):
			blockProtection = valueAfterColon(line)
		case strings.HasPrefix(lower, "tb"):
			topBottom = valueAfterColon(line)
		case strings.HasPrefix(lower, "qe"):
			quadMode = valueAfterColon(line)
		}
	}
	if flash.JEDECID == "" && unknownJEDEC != "" &&
		memoryType != "" && memoryCapacity != "" {
		flash.JEDECID = "0x" + strings.ToUpper(
			trimHexPrefix(unknownJEDEC)+
				trimHexPrefix(memoryType)+
				trimHexPrefix(memoryCapacity),
		)
		if capacityCode, err := strconv.ParseUint(trimHexPrefix(memoryCapacity), 16, 8); err == nil &&
			capacityCode >= 20 && capacityCode <= 40 {
			flash.CapacityMbit = (1 << capacityCode) * 8 / (1024 * 1024)
		}
	}
	if blockProtection != "" {
		flash.Protection = "BP=" + blockProtection
		if topBottom != "" {
			flash.Protection += ", TB=" + topBottom
		}
	}
	if quadMode != "" {
		if quadMode == "1" {
			flash.QuadEnabled = "Enabled"
		} else {
			flash.QuadEnabled = "Disabled"
		}
	}
	if flash.JEDECID == "" {
		return fpgaFlashInfo{}, fmt.Errorf("programmer did not return an SPI flash JEDEC ID")
	}
	return flash, nil
}

func formatFPGADeviceDetails(device deviceResult, snapshot *fpgaAdvancedSnapshot) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, "FPGA Detailed Info")
	for _, row := range fpgaDeviceDetailRows(device, snapshot) {
		fmt.Fprintf(&builder, "%s / %s: %s\n", row.Section, row.Field, row.Value)
	}
	return strings.TrimSpace(builder.String())
}

func fpgaIRLength(device deviceResult) string {
	if parsed, err := parseHexUint64(device.IDCode); err == nil {
		if part, ok := supportedJTAGPart(uint32(parsed)); ok {
			return fmt.Sprintf("%d bits", part.IRLength)
		}
	}
	family := partFamily(device.Part)
	for _, part := range supportedJTAGParts {
		if part.Family == family {
			return fmt.Sprintf("%d bits", part.IRLength)
		}
	}
	return "Unavailable"
}

func bitSet(value uint32, bit uint) bool {
	return value&(uint32(1)<<bit) != 0
}

func valueOrUnavailable(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unavailable"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "Complete"
	}
	return "Pending"
}

func highLow(value bool) string {
	if value {
		return "High"
	}
	return "Low"
}

func clearDetected(value bool) string {
	if value {
		return "Detected"
	}
	return "Clear"
}

func enabledDisabled(value bool) string {
	if value {
		return "Enabled"
	}
	return "Disabled"
}

func configurationBusWidth(value uint32) string {
	return map[uint32]string{0: "x1", 1: "x8", 2: "x16", 3: "x32"}[value]
}

func readbackSecurity(value uint32) string {
	switch value {
	case 0:
		return "Read/write enabled"
	case 1:
		return "Readback disabled"
	default:
		return "Read and write disabled"
	}
}

func formatVoltageRange(current, minimum, maximum float64) string {
	return fmt.Sprintf("%.3f V (min %.3f, max %.3f)", current, minimum, maximum)
}

func xadcVoltage(raw uint32) float64 {
	return float64(raw>>4) / 4096.0 * 3.0
}

func valueAfterColon(value string) string {
	if separator := strings.IndexByte(value, ':'); separator >= 0 {
		return strings.TrimSpace(value[separator+1:])
	}
	return ""
}

func normalizeHexLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "0x" + strings.ToUpper(trimHexPrefix(value))
}

func trimHexPrefix(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	value = strings.TrimPrefix(value, "0X")
	return value
}
