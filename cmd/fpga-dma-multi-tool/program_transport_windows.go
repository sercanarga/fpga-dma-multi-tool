//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
)

func prepareProgrammingTransport(
	ctx context.Context,
	request programRequest,
) (programTransport, error) {
	if request.Cable != directCH347ProgrammingCable {
		return programTransport{Cable: request.Cable}, nil
	}
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
