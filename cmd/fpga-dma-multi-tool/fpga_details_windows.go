//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func inspectFPGAAdvancedInfo(
	ctx context.Context,
	device deviceResult,
) (fpgaAdvancedSnapshot, error) {
	executable, transport, err := prepareFPGAInfoRuntime(ctx, device)
	if err != nil {
		return fpgaAdvancedSnapshot{}, err
	}
	if transport.Close != nil {
		defer transport.Close()
	}

	snapshot := fpgaAdvancedSnapshot{
		Registers: make(map[string]uint32, len(fpgaConfigurationRegisters)),
		Transport: transport.Description,
		DisruptiveReadNote: "Read sensors temporarily changes the XADC sequencer; " +
			"Probe flash temporarily loads a JTAG bridge and interrupts the active FPGA image.",
	}
	var readErrors []error
	for _, register := range fpgaConfigurationRegisters {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		args := fpgaInfoArguments(device, transport,
			"--read-register", register,
		)
		output, runErr := runOpenFPGALoaderCapture(ctx, executable, args)
		if runErr != nil {
			readErrors = append(readErrors, fmt.Errorf("%s: %w", register, runErr))
			snapshot.Warnings = append(
				snapshot.Warnings,
				fmt.Sprintf("%s could not be read.", register),
			)
			continue
		}
		value, parseErr := parseOpenFPGALoaderRegister(output)
		if parseErr != nil {
			readErrors = append(readErrors, fmt.Errorf("%s: %w", register, parseErr))
			snapshot.Warnings = append(
				snapshot.Warnings,
				fmt.Sprintf("%s returned an unsupported response.", register),
			)
			continue
		}
		snapshot.Registers[register] = value
	}
	if len(snapshot.Registers) == 0 {
		return snapshot, fmt.Errorf(
			"FPGA configuration registers could not be read: %w",
			errors.Join(readErrors...),
		)
	}
	return snapshot, nil
}

func inspectFPGAXADC(
	ctx context.Context,
	device deviceResult,
) (fpgaXADCInfo, string, error) {
	executable, transport, err := prepareFPGAInfoRuntime(ctx, device)
	if err != nil {
		return fpgaXADCInfo{}, "", err
	}
	if transport.Close != nil {
		defer transport.Close()
	}
	output, err := runOpenFPGALoaderCapture(
		ctx,
		executable,
		fpgaInfoArguments(device, transport, "--read-xadc"),
	)
	if err != nil {
		return fpgaXADCInfo{}, transport.Description, fmt.Errorf(
			"XADC read failed: %w\n%s",
			err,
			strings.TrimSpace(output),
		)
	}
	sensors, err := parseOpenFPGALoaderXADC(output)
	return sensors, transport.Description, err
}

func inspectFPGAFlash(
	ctx context.Context,
	device deviceResult,
) (fpgaFlashInfo, string, error) {
	executable, transport, err := prepareFPGAInfoRuntime(ctx, device)
	if err != nil {
		return fpgaFlashInfo{}, "", err
	}
	if transport.Close != nil {
		defer transport.Close()
	}
	args := fpgaInfoArguments(device, transport,
		"--detect",
		"--write-flash",
		"--fpga-part", partFamily(device.Part),
	)
	output, err := runOpenFPGALoaderCapture(ctx, executable, args)
	if err != nil {
		return fpgaFlashInfo{}, transport.Description, fmt.Errorf(
			"SPI flash probe failed: %w\n%s",
			err,
			strings.TrimSpace(output),
		)
	}
	flash, err := parseOpenFPGALoaderFlash(output)
	return flash, transport.Description, err
}

func prepareFPGAInfoRuntime(
	ctx context.Context,
	device deviceResult,
) (string, programTransport, error) {
	cable := autoProgrammingCable
	switch {
	case strings.Contains(strings.ToLower(device.Backend), "ch347"):
		cable = directCH347ProgrammingCable
	case strings.Contains(strings.ToLower(device.Backend), "rs232"):
		cable = rs232ProgrammingCable
	}
	transport, err := prepareProgrammingTransport(ctx, programRequest{Cable: cable})
	if err != nil {
		return "", programTransport{}, err
	}
	executable, err := findOpenFPGALoader("")
	if err != nil {
		if transport.Close != nil {
			_ = transport.Close()
		}
		return "", programTransport{}, err
	}
	if strings.TrimSpace(transport.Executable) != "" {
		executable = transport.Executable
	}
	return executable, transport, nil
}

func fpgaInfoArguments(
	device deviceResult,
	transport programTransport,
	operation ...string,
) []string {
	args := []string{"--cable", transport.Cable}
	args = append(args, operation...)
	args = append(args, transport.Arguments...)
	if device.Index > 0 {
		args = append(args, "--index-chain", strconv.Itoa(device.Index))
	}
	return args
}
