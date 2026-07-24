package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	xvcDefaultPeriodNS = uint32(2133)
	xvcMaxDevices      = 16
	fuseDNAInstruction = uint64(0x32)
)

type xvcClient struct {
	conn net.Conn
}

type jtagTransport interface {
	shift(tms, tdi []byte) ([]byte, error)
}

type jtagChainScanner struct {
	transport jtagTransport
}

type artixChainScanner interface {
	detectChain() ([]uint32, error)
	readFuseDNA(parts []jtagPart, target int) (uint64, error)
}

func scanXVC(ctx context.Context, opts options, diagnostics io.Writer) ([]deviceResult, error) {
	if _, _, err := net.SplitHostPort(opts.xvcAddress); err != nil {
		return nil, fmt.Errorf("invalid --xvc address %q: %w", opts.xvcAddress, err)
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", opts.xvcAddress)
	if err != nil {
		return nil, fmt.Errorf("connect to XVC server %s: %w", opts.xvcAddress, err)
	}
	defer conn.Close()
	client := &xvcClient{conn: conn}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set XVC deadline: %w", err)
		}
	}
	info, err := client.getInfo()
	if err != nil {
		return nil, err
	}
	if opts.verbose {
		fmt.Fprintf(diagnostics, "XVC server: %s\n", info)
	}
	if _, err := client.setTCK(xvcDefaultPeriodNS); err != nil {
		return nil, err
	}
	return scanDirectArtix(client, opts.device, opts.xvcAddress, "xvc-direct")
}

func scanDirectArtix(
	transport jtagTransport,
	selectedDevice int,
	target string,
	backend string,
) ([]deviceResult, error) {
	return scanArtixDevices(
		&jtagChainScanner{transport: transport},
		selectedDevice,
		target,
		backend,
	)
}

func scanArtixDevices(
	scanner artixChainScanner,
	selectedDevice int,
	target string,
	backend string,
) ([]deviceResult, error) {
	idcodes, err := scanner.detectChain()
	if err != nil {
		return nil, err
	}
	if len(idcodes) == 0 {
		return nil, errors.New("JTAG transport responded, but no devices were detected")
	}
	if selectedDevice >= len(idcodes) {
		return nil, fmt.Errorf("--device %d is outside the detected JTAG chain (devices: %d)", selectedDevice, len(idcodes))
	}
	if selectedDevice < -1 {
		return nil, errors.New("--device must be -1 or a zero-based JTAG chain index")
	}

	parts := make([]jtagPart, len(idcodes))
	for index, idcode := range idcodes {
		part, ok := supportedJTAGPart(idcode)
		if !ok {
			return nil, fmt.Errorf(
				"JTAG device %d has unsupported IDCODE 0x%08X; only supported Artix-7 chains can be read",
				index, idcode,
			)
		}
		parts[index] = part
	}

	var results []deviceResult
	for index, idcode := range idcodes {
		if selectedDevice >= 0 && index != selectedDevice {
			continue
		}
		first, err := scanner.readFuseDNA(parts, index)
		if err != nil {
			return nil, fmt.Errorf("read device %d FUSE_DNA: %w", index, err)
		}
		second, err := scanner.readFuseDNA(parts, index)
		if err != nil {
			return nil, fmt.Errorf("verify device %d FUSE_DNA: %w", index, err)
		}
		if first != second {
			return nil, fmt.Errorf(
				"unstable FUSE_DNA read for device %d: %s then %s; reduce XVC/JTAG clock or check the cable",
				index, formatFuseDNA(first), formatFuseDNA(second),
			)
		}
		if first == 0 || first == ^uint64(0) {
			return nil, fmt.Errorf("device %d returned invalid FUSE_DNA %s", index, formatFuseDNA(first))
		}
		part := parts[index]
		results = append(results, deviceResult{
			Index:        index,
			Target:       target,
			Name:         part.Name,
			Part:         part.Name,
			IDCode:       fmt.Sprintf("0x%08X", idcode),
			FuseDNA:      formatFuseDNA(first),
			DeviceDNA:    formatDeviceDNA(first),
			BoardMatches: chainBoardMatches(idcodes, part.Family),
			Backend:      backend,
		})
	}
	return results, nil
}

func supportedJTAGPart(idcode uint32) (jtagPart, bool) {
	// IEEE 1149.1 reserves IDCODE[31:28] for the silicon version. The
	// manufacturer, part and family identity is held in the lower 28 bits.
	part, ok := supportedJTAGParts[idcode&0x0FFFFFFF]
	return part, ok
}

func (client *xvcClient) getInfo() (string, error) {
	if err := writeAll(client.conn, []byte("getinfo:")); err != nil {
		return "", fmt.Errorf("send XVC getinfo: %w", err)
	}
	info, err := bufio.NewReader(client.conn).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read XVC getinfo: %w", err)
	}
	info = strings.TrimSpace(info)
	if !strings.HasPrefix(info, "xvcServer_v1.0:") {
		return "", fmt.Errorf("unexpected XVC server response %q", info)
	}
	return info, nil
}

func (client *xvcClient) setTCK(periodNS uint32) (uint32, error) {
	payload := make([]byte, len("settck:")+4)
	copy(payload, "settck:")
	binary.LittleEndian.PutUint32(payload[len("settck:"):], periodNS)
	if err := writeAll(client.conn, payload); err != nil {
		return 0, fmt.Errorf("send XVC settck: %w", err)
	}
	var response [4]byte
	if _, err := io.ReadFull(client.conn, response[:]); err != nil {
		return 0, fmt.Errorf("read XVC settck: %w", err)
	}
	return binary.LittleEndian.Uint32(response[:]), nil
}

func (scanner *jtagChainScanner) resetToIdle() error {
	tms := []byte{1, 1, 1, 1, 1, 1, 0}
	tdi := make([]byte, len(tms))
	_, err := scanner.transport.shift(tms, tdi)
	return err
}

func (scanner *jtagChainScanner) detectChain() ([]uint32, error) {
	if err := scanner.resetToIdle(); err != nil {
		return nil, err
	}
	tdo, err := scanner.scanDR(make([]byte, 32*xvcMaxDevices))
	if err != nil {
		return nil, fmt.Errorf("scan JTAG IDCODE chain: %w", err)
	}
	var idcodes []uint32
	for index := 0; index < xvcMaxDevices; index++ {
		value := uint32(readBitsLSB(tdo, index*32, 32))
		if value == 0 || value == ^uint32(0) {
			if index == 0 {
				level := "low"
				if value == ^uint32(0) {
					level = "high"
				}
				return nil, fmt.Errorf(
					"no JTAG IDCODE was returned; TDO remained %s for the first 32 clocks (0x%08X)",
					level,
					value,
				)
			}
			break
		}
		if value&1 == 0 {
			return nil, fmt.Errorf(
				"JTAG chain is not an IDCODE-only chain at bit %d (value 0x%08X)",
				index*32, value,
			)
		}
		idcodes = append(idcodes, value)
	}
	return idcodes, nil
}

func (scanner *jtagChainScanner) readFuseDNA(parts []jtagPart, target int) (uint64, error) {
	if err := scanner.resetToIdle(); err != nil {
		return 0, err
	}
	var instructions []byte
	for index, part := range parts {
		instruction := uint64((1 << part.IRLength) - 1)
		if index == target {
			instruction = fuseDNAInstruction
		}
		instructions = append(instructions, uintToBits(instruction, part.IRLength)...)
	}
	if _, err := scanner.scanIR(instructions); err != nil {
		return 0, fmt.Errorf("select FUSE_DNA instruction: %w", err)
	}

	// Every non-selected device is in BYPASS and contributes one DR bit.
	drLength := 64 + len(parts) - 1
	tdo, err := scanner.scanDR(make([]byte, drLength))
	if err != nil {
		return 0, fmt.Errorf("shift FUSE_DNA register: %w", err)
	}
	// Chain index zero is closest to TDO, so preceding BYPASS devices
	// contribute one bit before the selected device's 64-bit register.
	return readBitsLSB(tdo, target, 64), nil
}

func (scanner *jtagChainScanner) scanIR(data []byte) ([]byte, error) {
	return scanner.scanRegister([]byte{1, 1, 0, 0}, data)
}

func (scanner *jtagChainScanner) scanDR(data []byte) ([]byte, error) {
	return scanner.scanRegister([]byte{1, 0, 0}, data)
}

func (scanner *jtagChainScanner) scanRegister(prefix []byte, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot scan an empty JTAG register")
	}
	tms := append([]byte(nil), prefix...)
	tdi := make([]byte, len(prefix))
	for index, bit := range data {
		exit := byte(0)
		if index == len(data)-1 {
			exit = 1
		}
		tms = append(tms, exit)
		tdi = append(tdi, bit)
	}
	tms = append(tms, 1, 0)
	tdi = append(tdi, 0, 0)
	tdo, err := scanner.transport.shift(tms, tdi)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), tdo[len(prefix):len(prefix)+len(data)]...), nil
}

func (client *xvcClient) shift(tms, tdi []byte) ([]byte, error) {
	if len(tms) != len(tdi) {
		return nil, errors.New("internal XVC TMS/TDI length mismatch")
	}
	byteCount := (len(tms) + 7) / 8
	payload := make([]byte, len("shift:")+4+2*byteCount)
	copy(payload, "shift:")
	binary.LittleEndian.PutUint32(payload[len("shift:"):], uint32(len(tms)))
	copy(payload[len("shift:")+4:], packBits(tms))
	copy(payload[len("shift:")+4+byteCount:], packBits(tdi))
	if err := writeAll(client.conn, payload); err != nil {
		return nil, fmt.Errorf("send XVC shift: %w", err)
	}
	response := make([]byte, byteCount)
	if _, err := io.ReadFull(client.conn, response); err != nil {
		return nil, fmt.Errorf("read XVC shift: %w", err)
	}
	return unpackBits(response, len(tms)), nil
}

func packBits(bits []byte) []byte {
	packed := make([]byte, (len(bits)+7)/8)
	for index, bit := range bits {
		if bit != 0 {
			packed[index/8] |= 1 << (index % 8)
		}
	}
	return packed
}

func unpackBits(packed []byte, count int) []byte {
	bits := make([]byte, count)
	for index := range bits {
		bits[index] = (packed[index/8] >> (index % 8)) & 1
	}
	return bits
}

func uintToBits(value uint64, count int) []byte {
	bits := make([]byte, count)
	for index := range bits {
		bits[index] = byte((value >> index) & 1)
	}
	return bits
}

func readBitsLSB(bits []byte, offset, count int) uint64 {
	var value uint64
	for index := 0; index < count; index++ {
		if bits[offset+index] != 0 {
			value |= 1 << index
		}
	}
	return value
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) != 0 {
		written, err := w.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
