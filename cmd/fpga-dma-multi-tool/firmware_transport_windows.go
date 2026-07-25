//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func prepareProgrammingTransport(
	ctx context.Context,
	request programRequest,
) (programTransport, error) {
	switch request.Cable {
	case autoProgrammingCable:
		ch347Transport, ch347Err := prepareCH347ProgrammingTransport(ctx)
		if ch347Err == nil {
			return ch347Transport, nil
		}
		rs232Transport, rs232Err := prepareRS232ProgrammingTransport(ctx)
		if rs232Err == nil {
			return rs232Transport, nil
		}
		return programTransport{}, fmt.Errorf(
			"no supported programmer is ready (CH347: %v; RS232: %v)",
			ch347Err, rs232Err,
		)
	case directCH347ProgrammingCable:
		return prepareCH347ProgrammingTransport(ctx)
	case rs232ProgrammingCable:
		return prepareRS232ProgrammingTransport(ctx)
	default:
		return programTransport{Cable: request.Cable}, nil
	}
}

func prepareCH347ProgrammingTransport(ctx context.Context) (programTransport, error) {
	library, _, err := loadCH347Library("")
	if err != nil {
		return programTransport{}, err
	}
	adapterIndex := -1
	for index := 0; index < ch347MaxAdapters; index++ {
		if library.openDevice(uint32(index)) {
			adapterIndex = index
			break
		}
	}
	if adapterIndex < 0 {
		library.dll.Release()
		return programTransport{}, errors.New("no CH347 adapter is connected")
	}
	closeAdapter := func() {
		library.closeDevice(uint32(adapterIndex))
		library.dll.Release()
	}
	if err := library.configure(uint32(adapterIndex)); err != nil {
		closeAdapter()
		return programTransport{}, err
	}
	bridge, err := startXVCBridge(ctx, &ch347Transport{
		ctx: ctx, library: library, index: uint32(adapterIndex),
	})
	if err != nil {
		closeAdapter()
		return programTransport{}, err
	}
	host, port, err := net.SplitHostPort(bridge.Address())
	if err != nil {
		_ = bridge.Close()
		closeAdapter()
		return programTransport{}, fmt.Errorf("parse local XVC address: %w", err)
	}
	return programTransport{
		Cable: "xvc-client",
		Arguments: []string{
			"--ip", host,
			"--port", port,
		},
		Close: func() error {
			bridgeErr := bridge.Close()
			closeAdapter()
			return bridgeErr
		},
		Description: "CH347[" + strconv.Itoa(adapterIndex) + "] via local XVC",
	}, nil
}

func prepareRS232ProgrammingTransport(ctx context.Context) (programTransport, error) {
	devices, err := detectRS232Devices(ctx)
	if err != nil {
		return programTransport{}, err
	}
	if len(devices) == 0 {
		return programTransport{}, errors.New("no supported RS232 writer is connected")
	}
	var ready []rs232Device
	for _, device := range devices {
		if rs232ServiceReady(device.Service) {
			ready = append(ready, device)
		}
	}
	if len(ready) == 0 {
		return programTransport{}, errors.New(
			"RS232 writer Interface 0 needs the WinUSB driver; install it from the Drivers tab",
		)
	}
	executable, err := findOpenFPGALoader("")
	if err != nil {
		return programTransport{}, err
	}

	var failures []string
	for _, device := range ready {
		candidates, candidateErr := rs232CableCandidates(device)
		if candidateErr != nil {
			failures = append(failures, candidateErr.Error())
			continue
		}
		for _, cable := range candidates {
			transport := rs232TransportForDevice(device, cable)
			probeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			args := []string{"--detect", "--cable", transport.Cable}
			args = append(args, transport.Arguments...)
			output, probeErr := runOpenFPGALoaderCapture(probeContext, executable, args)
			cancel()
			if probeErr == nil {
				if _, parseErr := parseOpenFPGALoaderChain(output); parseErr == nil {
					return transport, nil
				}
			}
			failures = append(
				failures,
				fmt.Sprintf("%s: %s", cable, rs232ProbeFailure(probeErr, output)),
			)
		}
	}
	return programTransport{}, fmt.Errorf(
		"RS232 writer driver is ready, but no JTAG cable profile could read the FPGA (%s)",
		strings.Join(failures, "; "),
	)
}

func rs232TransportForDevice(device rs232Device, cable string) programTransport {
	description := strings.TrimSpace(device.Name)
	if description == "" {
		description = "FTDI writer"
	}
	return programTransport{
		Cable:     cable,
		Arguments: rs232DeviceArguments(device),
		Description: fmt.Sprintf(
			"%s (%s, 0403:%s, Interface A, %s)",
			description, device.Service, device.PID, cable,
		),
	}
}

func rs232ProbeFailure(probeErr error, output string) string {
	detail := strings.Join(strings.Fields(strings.TrimSpace(output)), " ")
	if detail == "" && probeErr != nil {
		detail = probeErr.Error()
	}
	if len(detail) > 240 {
		detail = detail[:240] + "…"
	}
	if detail == "" {
		return "no JTAG device was reported"
	}
	return detail
}
