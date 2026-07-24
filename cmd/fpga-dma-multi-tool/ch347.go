package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var errCH347Unavailable = errors.New("CH347 JTAG adapter is unavailable")

const (
	ch347MaxAdapters        = 16
	ch347MaxBitsPerScan     = 248
	ch347JTAGClockIndex     = 0 // 468.75 kHz, reliable on every supported revision.
	ch347CommandJTAGBitRead = byte(0xD2)
	ch347PinTRSTHigh        = byte(0x20)
)

func scanAutomatic(ctx context.Context, opts options, diagnostics io.Writer) ([]deviceResult, error) {
	return scanCH347(ctx, opts, diagnostics)
}

func buildCH347BitCommand(tms, tdi []byte) ([]byte, error) {
	if len(tms) != len(tdi) {
		return nil, errors.New("internal CH347 TMS/TDI length mismatch")
	}
	if len(tms) == 0 || len(tms) > ch347MaxBitsPerScan {
		return nil, fmt.Errorf("CH347 bit command length must be 1-%d, got %d", ch347MaxBitsPerScan, len(tms))
	}
	command := make([]byte, 3+2*len(tms)+1)
	command[0] = ch347CommandJTAGBitRead
	binary.LittleEndian.PutUint16(command[1:3], uint16(2*len(tms)+1))
	var lastState byte
	for bit := range tms {
		// TRST is active-low on the CH347 JTAG interface. Keep it deasserted
		// while clocking the TAP; otherwise every transfer holds the chain in
		// reset and TDO never returns an IDCODE.
		state := ch347PinTRSTHigh
		if tms[bit] != 0 {
			state |= 0x02
		}
		if tdi[bit] != 0 {
			state |= 0x10
		}
		command[3+2*bit] = state
		command[3+2*bit+1] = state | 0x01
		lastState = state
	}
	command[len(command)-1] = lastState
	return command, nil
}

func decodeCH347BitResponse(response []byte, bitCount int) ([]byte, error) {
	if bitCount <= 0 || bitCount > ch347MaxBitsPerScan {
		return nil, fmt.Errorf("invalid CH347 response bit count %d", bitCount)
	}
	if len(response) != 3+bitCount {
		return nil, fmt.Errorf("CH347 response length is %d, expected %d", len(response), 3+bitCount)
	}
	if response[0] != ch347CommandJTAGBitRead {
		return nil, fmt.Errorf(
			"CH347 response command is 0x%02X, expected 0x%02X",
			response[0], ch347CommandJTAGBitRead,
		)
	}
	reported := int(binary.LittleEndian.Uint16(response[1:3]))
	if reported != bitCount {
		return nil, fmt.Errorf("CH347 response payload length is %d, expected %d", reported, bitCount)
	}
	tdo := make([]byte, bitCount)
	for bit := range tdo {
		tdo[bit] = response[3+bit] & 1
	}
	return tdo, nil
}
