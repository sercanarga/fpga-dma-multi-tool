package main

import (
	"fmt"
	"strconv"
	"strings"
)

type jtagPart struct {
	Name     string
	Family   string
	IRLength int
}

var supportedJTAGParts = map[uint32]jtagPart{
	0x0362e093: {Name: "xc7a15t", Family: "xc7a15t", IRLength: 6},
	0x0362d093: {Name: "xc7a35t", Family: "xc7a35t", IRLength: 6},
	0x0362c093: {Name: "xc7a50t", Family: "xc7a50t", IRLength: 6},
	0x03632093: {Name: "xc7a75t", Family: "xc7a75t", IRLength: 6},
	0x03631093: {Name: "xc7a100t", Family: "xc7a100t", IRLength: 6},
	0x03636093: {Name: "xc7a200t", Family: "xc7a200t", IRLength: 6},
}

var boardNamesByFamily = map[string][]string{
	"xc7a35t": {
		"PCIeSquirrel",
		"ScreamerM2",
		"pciescreamer",
		"CaptainDMA_M2_x1",
		"CaptainDMA_M2_x4",
		"CaptainDMA_35T",
		"GBOX",
		"NeTV2_35T",
	},
	"xc7a75t": {
		"EnigmaX1",
		"CaptainDMA_75T",
	},
	"xc7a100t": {
		"CaptainDMA_100T",
		"ZDMA",
		"NeTV2_100T",
		"litefury",
	},
	"xc7a200t": {
		"ac701_ft601",
		"acorn",
	},
}

func parseHexUint64(value string) (uint64, error) {
	clean := strings.TrimSpace(value)
	clean = strings.TrimPrefix(clean, "0x")
	clean = strings.TrimPrefix(clean, "0X")
	clean = strings.ReplaceAll(clean, "_", "")
	if clean == "" {
		return 0, fmt.Errorf("empty hexadecimal value")
	}
	parsed, err := strconv.ParseUint(clean, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hexadecimal value %q: %w", value, err)
	}
	return parsed, nil
}

func formatFuseDNA(value uint64) string {
	return fmt.Sprintf("0x%016X", value)
}

func formatDeviceDNA(value uint64) string {
	return fmt.Sprintf("0x%015X", value>>7)
}

func boardMatches(part string) []string {
	family := partFamily(part)
	if family == "" {
		return nil
	}
	return append([]string(nil), boardNamesByFamily[family]...)
}

func chainBoardMatches(idcodes []uint32, part string) []string {
	matches := boardMatches(part)
	if isZDMAChain(idcodes) && !stringSliceContains(matches, "ZDMA") {
		matches = append(matches, "ZDMA")
	}
	return matches
}

func isZDMAChain(idcodes []uint32) bool {
	if len(idcodes) != 2 {
		return false
	}
	const (
		artix75T  = uint32(0x03632093)
		artix100T = uint32(0x03631093)
	)
	first := idcodes[0] & 0x0FFFFFFF
	second := idcodes[1] & 0x0FFFFFFF
	return (first == artix75T && second == artix100T) ||
		(first == artix100T && second == artix75T)
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func partFamily(part string) string {
	part = strings.ToLower(strings.TrimSpace(part))
	for _, family := range []string{
		"xc7a15t", "xc7a25t", "xc7a35t", "xc7a50t",
		"xc7a75t", "xc7a100t", "xc7a200t",
	} {
		if strings.Contains(part, family) {
			return family
		}
	}
	return ""
}
