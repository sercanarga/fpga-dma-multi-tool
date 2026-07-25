//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
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
	var selected *rs232Device
	for index := range devices {
		if rs232ServiceReady(devices[index].Service) {
			selected = &devices[index]
			break
		}
	}
	if selected == nil {
		return programTransport{}, errors.New(
			"RS232 writer Interface 0 needs the WinUSB driver; install it from the Drivers tab",
		)
	}
	cable, err := rs232CableForPID(selected.PID)
	if err != nil {
		return programTransport{}, err
	}
	description := strings.TrimSpace(selected.Name)
	if description == "" {
		description = "FTDI RS232 writer"
	}
	return programTransport{
		Cable: cable,
		Arguments: []string{
			"--vid", "0x0403",
			"--pid", "0x" + selected.PID,
			"--ftdi-channel", "0",
		},
		Description: fmt.Sprintf(
			"%s (%s, 0403:%s, Interface A)",
			description, selected.Service, selected.PID,
		),
	}, nil
}
