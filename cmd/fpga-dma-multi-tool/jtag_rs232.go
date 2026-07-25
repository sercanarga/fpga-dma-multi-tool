package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type openFPGALoaderDevice struct {
	Index  int
	IDCode uint32
}

func parseOpenFPGALoaderChain(output string) ([]openFPGALoaderDevice, error) {
	var devices []openFPGALoaderDevice
	currentIndex := -1
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) == 2 && strings.EqualFold(fields[0], "index") &&
			strings.HasSuffix(fields[1], ":") {
			value := strings.TrimSuffix(fields[1], ":")
			index, err := strconv.Atoi(value)
			if err != nil || index < 0 {
				return nil, fmt.Errorf("invalid JTAG chain index %q", value)
			}
			currentIndex = index
			continue
		}
		if currentIndex < 0 || len(fields) != 2 || !strings.EqualFold(fields[0], "idcode") {
			continue
		}
		parsed, err := parseHexUint64(fields[1])
		if err != nil || parsed > uint64(^uint32(0)) {
			return nil, fmt.Errorf("invalid IDCODE for JTAG device %d: %q", currentIndex, fields[1])
		}
		devices = append(devices, openFPGALoaderDevice{
			Index:  currentIndex,
			IDCode: uint32(parsed),
		})
		currentIndex = -1
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read programmer output: %w", err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("programmer responded, but no JTAG devices were reported")
	}
	for index, device := range devices {
		if device.Index != index {
			return nil, fmt.Errorf(
				"programmer returned a non-contiguous JTAG chain at index %d",
				device.Index,
			)
		}
	}
	return devices, nil
}

func parseOpenFPGALoaderDNA(output string) (uint64, error) {
	const marker = `"dna"`
	position := strings.Index(strings.ToLower(output), marker)
	if position < 0 {
		return 0, fmt.Errorf("programmer did not return an FPGA DNA value")
	}
	remainder := output[position+len(marker):]
	colon := strings.IndexByte(remainder, ':')
	if colon < 0 {
		return 0, fmt.Errorf("programmer returned malformed FPGA DNA output")
	}
	value := strings.TrimSpace(remainder[colon+1:])
	value = strings.TrimLeft(value, `"`)
	end := strings.IndexAny(value, `"}`+"\r\n")
	if end >= 0 {
		value = value[:end]
	}
	dna, err := parseHexUint64(value)
	if err != nil {
		return 0, fmt.Errorf("parse programmer DNA: %w", err)
	}
	if dna == 0 || dna == ^uint64(0) {
		return 0, fmt.Errorf("programmer returned invalid FPGA DNA %s", formatFuseDNA(dna))
	}
	return dna, nil
}
