//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func scanRS232(ctx context.Context, opts options, diagnostics io.Writer) ([]deviceResult, error) {
	executable, err := findRS232OpenFPGALoader()
	if err != nil {
		return nil, err
	}
	transport, err := prepareRS232ProgrammingTransport(ctx)
	if err != nil {
		return nil, err
	}
	if transport.Close != nil {
		defer transport.Close()
	}
	if transport.Description != "" {
		fmt.Fprintf(diagnostics, "Transport: %s\n", transport.Description)
	}

	detectArgs := []string{"--detect", "--cable", transport.Cable}
	detectArgs = append(detectArgs, transport.Arguments...)
	detectOutput, err := runOpenFPGALoaderCapture(ctx, executable, detectArgs)
	fmt.Fprint(diagnostics, detectOutput)
	if err != nil {
		return nil, fmt.Errorf(
			"RS232 JTAG detection failed: %w\n%s",
			err, strings.TrimSpace(detectOutput),
		)
	}
	chain, err := parseOpenFPGALoaderChain(detectOutput)
	if err != nil {
		return nil, err
	}
	if opts.device < -1 || opts.device >= len(chain) {
		return nil, fmt.Errorf(
			"--device %d is outside the detected JTAG chain (devices: %d)",
			opts.device, len(chain),
		)
	}

	idcodes := make([]uint32, len(chain))
	parts := make([]jtagPart, len(chain))
	for index, device := range chain {
		idcodes[index] = device.IDCode
		part, ok := supportedJTAGPart(device.IDCode)
		if !ok {
			return nil, fmt.Errorf(
				"JTAG device %d has unsupported IDCODE 0x%08X; only supported Artix-7 chains can be read",
				index, device.IDCode,
			)
		}
		parts[index] = part
	}

	var results []deviceResult
	for index, device := range chain {
		if opts.device >= 0 && index != opts.device {
			continue
		}
		first, err := readOpenFPGALoaderDNA(
			ctx, executable, transport, device.Index, diagnostics,
		)
		if err != nil {
			return nil, fmt.Errorf("read device %d FUSE_DNA: %w", device.Index, err)
		}
		second, err := readOpenFPGALoaderDNA(
			ctx, executable, transport, device.Index, diagnostics,
		)
		if err != nil {
			return nil, fmt.Errorf("verify device %d FUSE_DNA: %w", device.Index, err)
		}
		if first != second {
			return nil, fmt.Errorf(
				"unstable FUSE_DNA read for device %d: %s then %s; check the cable and board power",
				device.Index, formatFuseDNA(first), formatFuseDNA(second),
			)
		}
		part := parts[index]
		results = append(results, deviceResult{
			Index:        device.Index,
			Target:       transport.Description,
			Name:         part.Name,
			Part:         part.Name,
			IDCode:       fmt.Sprintf("0x%08X", device.IDCode),
			FuseDNA:      formatFuseDNA(first),
			DeviceDNA:    formatDeviceDNA(first),
			BoardMatches: chainBoardMatches(idcodes, part.Family),
			Backend:      "rs232-openfpgaloader",
		})
	}
	return results, nil
}

func readOpenFPGALoaderDNA(
	ctx context.Context,
	executable string,
	transport programTransport,
	index int,
	diagnostics io.Writer,
) (uint64, error) {
	args := []string{"--read-dna", "--cable", transport.Cable}
	args = append(args, transport.Arguments...)
	if index > 0 {
		args = append(args, "--index-chain", strconv.Itoa(index))
	}
	output, err := runOpenFPGALoaderCapture(ctx, executable, args)
	fmt.Fprint(diagnostics, output)
	if err != nil {
		return 0, fmt.Errorf("%w\n%s", err, strings.TrimSpace(output))
	}
	return parseOpenFPGALoaderDNA(output)
}

func runOpenFPGALoaderCapture(
	ctx context.Context,
	executable string,
	args []string,
) (string, error) {
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = openFPGALoaderEnvironment(executable)
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	return string(output), err
}
