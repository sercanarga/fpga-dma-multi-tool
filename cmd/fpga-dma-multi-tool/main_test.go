package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBitPackingRoundTrip(t *testing.T) {
	bits := []byte{1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1}
	got := unpackBits(packBits(bits), len(bits))
	if !bytes.Equal(got, bits) {
		t.Fatalf("round trip = %v, want %v", got, bits)
	}
}

func TestReadBitsLSB(t *testing.T) {
	bits := append([]byte{0, 1}, uintToBits(0x0123456789ABCDEF, 64)...)
	if got := readBitsLSB(bits, 2, 64); got != 0x0123456789ABCDEF {
		t.Fatalf("readBitsLSB() = 0x%016X", got)
	}
}

func TestSelectBackend(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		want    string
		wantErr bool
	}{
		{name: "auto", opts: options{backend: "auto"}, want: "auto"},
		{name: "auto xvc", opts: options{backend: "auto", xvcAddress: "localhost:2542"}, want: "xvc"},
		{name: "explicit xvc", opts: options{backend: "xvc", xvcAddress: "localhost:2542"}, want: "xvc"},
		{name: "explicit ch347", opts: options{backend: "ch347"}, want: "ch347"},
		{name: "explicit rs232", opts: options{backend: "rs232"}, want: "rs232"},
		{name: "missing xvc address", opts: options{backend: "xvc"}, wantErr: true},
		{name: "unsupported backend", opts: options{backend: "removed"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := selectBackend(test.opts)
			if (err != nil) != test.wantErr {
				t.Fatalf("selectBackend() error = %v", err)
			}
			if got != test.want {
				t.Errorf("selectBackend() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseOpenFPGALoaderChain(t *testing.T) {
	output := `index 0:
	idcode 0x13631093
	manufacturer xilinx
	family artix a7 100t
	model  xc7a100
	irlength 6
index 1:
	idcode 0x03632093
	manufacturer xilinx
	family artix a7 75t
	model  xc7a75
	irlength 6
`
	devices, err := parseOpenFPGALoaderChain(output)
	if err != nil {
		t.Fatalf("parseOpenFPGALoaderChain() error = %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("device count = %d, want 2", len(devices))
	}
	if devices[0].Index != 0 || devices[0].IDCode != 0x13631093 {
		t.Fatalf("device 0 = %+v", devices[0])
	}
	if devices[1].Index != 1 || devices[1].IDCode != 0x03632093 {
		t.Fatalf("device 1 = %+v", devices[1])
	}
}

func TestParseOpenFPGALoaderChainRejectsMissingIndex(t *testing.T) {
	output := "index 1:\n\tidcode 0x03631093\n"
	if _, err := parseOpenFPGALoaderChain(output); err == nil {
		t.Fatal("parseOpenFPGALoaderChain() accepted a non-contiguous chain")
	}
}

func TestParseOpenFPGALoaderDNA(t *testing.T) {
	dna, err := parseOpenFPGALoaderDNA(
		"programmer details\r\n{\"dna\": \"0x0123456789abcdef\"}\r\n",
	)
	if err != nil {
		t.Fatalf("parseOpenFPGALoaderDNA() error = %v", err)
	}
	if dna != 0x0123456789ABCDEF {
		t.Fatalf("DNA = 0x%016X", dna)
	}
}

func TestCH347BitCommandAndResponse(t *testing.T) {
	tms := []byte{1, 0, 1}
	tdi := []byte{0, 1, 1}
	command, err := buildCH347BitCommand(tms, tdi)
	if err != nil {
		t.Fatalf("buildCH347BitCommand() error = %v", err)
	}
	wantCommand := []byte{
		0xD2, 0x07, 0x00,
		0x22, 0x23,
		0x30, 0x31,
		0x32, 0x33,
		0x32,
	}
	if !bytes.Equal(command, wantCommand) {
		t.Fatalf("command = % X, want % X", command, wantCommand)
	}
	for index, state := range command[3:] {
		if state&ch347PinTRSTHigh == 0 {
			t.Fatalf("command state %d asserts TRST: 0x%02X", index, state)
		}
	}
	response := []byte{0xD2, 0x03, 0x00, 0x01, 0x00, 0x01}
	got, err := decodeCH347BitResponse(response, 3)
	if err != nil {
		t.Fatalf("decodeCH347BitResponse() error = %v", err)
	}
	if want := []byte{1, 0, 1}; !bytes.Equal(got, want) {
		t.Fatalf("TDO = %v, want %v", got, want)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsConflictingProgramActions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"--programmer-check", "--program-file", "firmware.bit"},
		&stdout,
		&stderr,
	)
	if code != 2 || !strings.Contains(stderr.String(), "cannot be combined") {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunRejectsInvalidProgramModeBeforeOpeningAdapter(t *testing.T) {
	bitstream := filepath.Join(t.TempDir(), "firmware.bit")
	if err := os.WriteFile(bitstream, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(
		[]string{"--program-file", bitstream, "--program-mode", "invalid"},
		&stdout,
		&stderr,
	)
	if code != 1 || !strings.Contains(stderr.String(), "select a programming mode") {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
}

func TestDeviceScanMessagesAreSpecific(t *testing.T) {
	if got := deviceScanErrorMessage(errors.New("TDO remained high")); !strings.Contains(got, "TDO stayed high") {
		t.Fatalf("high TDO message = %q", got)
	}
	if got := deviceScanErrorMessage(errors.New("CH347 adapter not found")); !strings.Contains(got, "CH347 adapter not found") {
		t.Fatalf("missing adapter message = %q", got)
	}
	if got := deviceScanSummary(nil); got != "No device found" {
		t.Fatalf("empty summary = %q", got)
	}
}

func TestScanXVCReadsEveryArtixDeviceTwice(t *testing.T) {
	idcodes := []uint32{0x03632093, 0x03631093}
	dnaValues := []uint64{0x123456789ABCDEF0, 0xFEDCBA9876543210}
	address, closeServer := startFakeXVCServer(t, idcodes, dnaValues)
	defer closeServer()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results, err := scanXVC(ctx, options{xvcAddress: address, device: -1}, io.Discard)
	if err != nil {
		t.Fatalf("scanXVC() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for index, result := range results {
		if result.FuseDNA != formatFuseDNA(dnaValues[index]) {
			t.Errorf("result[%d].FuseDNA = %s, want %s", index, result.FuseDNA, formatFuseDNA(dnaValues[index]))
		}
		if result.IDCode != fmt.Sprintf("0x%08X", idcodes[index]) {
			t.Errorf("result[%d].IDCode = %s", index, result.IDCode)
		}
	}
	if !containsString(results[1].BoardMatches, "ZDMA") {
		t.Errorf("100T BoardMatches = %v, want ZDMA", results[1].BoardMatches)
	}
	if !containsString(results[0].BoardMatches, "ZDMA") {
		t.Errorf("75T BoardMatches = %v, want ZDMA", results[0].BoardMatches)
	}
}

func TestScanXVCIgnoresIDCODEVersionNibble(t *testing.T) {
	idcodes := []uint32{0x13631093}
	dnaValues := []uint64{0x123456789ABCDEF0}
	address, closeServer := startFakeXVCServer(t, idcodes, dnaValues)
	defer closeServer()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results, err := scanXVC(ctx, options{xvcAddress: address, device: -1}, io.Discard)
	if err != nil {
		t.Fatalf("scanXVC() error = %v", err)
	}
	if len(results) != 1 || results[0].Part != "xc7a100t" {
		t.Fatalf("versioned IDCODE result = %+v", results)
	}
	if results[0].IDCode != "0x13631093" {
		t.Fatalf("reported IDCODE = %q", results[0].IDCode)
	}
}

func startFakeXVCServer(t *testing.T, idcodes []uint32, dnaValues []uint64) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		done <- serveFakeXVC(conn, idcodes, dnaValues)
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		select {
		case serverErr := <-done:
			if serverErr != nil && !strings.Contains(serverErr.Error(), "closed") {
				t.Errorf("fake XVC server: %v", serverErr)
			}
		case <-time.After(time.Second):
			t.Error("fake XVC server did not stop")
		}
	}
}

func serveFakeXVC(conn net.Conn, idcodes []uint32, dnaValues []uint64) error {
	reader := bufio.NewReader(conn)
	var selected = -1
	for {
		command, err := reader.Peek(6)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch string(command) {
		case "getinf":
			request := make([]byte, len("getinfo:"))
			if _, err := io.ReadFull(reader, request); err != nil {
				return err
			}
			if string(request) != "getinfo:" {
				return fmt.Errorf("unexpected request %q", request)
			}
			if err := writeAll(conn, []byte("xvcServer_v1.0:4096\n")); err != nil {
				return err
			}
		case "settck":
			request := make([]byte, len("settck:")+4)
			if _, err := io.ReadFull(reader, request); err != nil {
				return err
			}
			if err := writeAll(conn, request[len("settck:"):]); err != nil {
				return err
			}
		case "shift:":
			header := make([]byte, len("shift:")+4)
			if _, err := io.ReadFull(reader, header); err != nil {
				return err
			}
			bitCount := int(binary.LittleEndian.Uint32(header[len("shift:"):]))
			byteCount := (bitCount + 7) / 8
			payload := make([]byte, 2*byteCount)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return err
			}
			tdi := unpackBits(payload[byteCount:], bitCount)
			tdo := make([]byte, bitCount)
			switch {
			case bitCount == 3+32*xvcMaxDevices+2:
				for index, idcode := range idcodes {
					copy(tdo[3+32*index:], uintToBits(uint64(idcode), 32))
				}
			case bitCount == 4+6*len(idcodes)+2:
				selected = -1
				for index := range idcodes {
					if readBitsLSB(tdi, 4+6*index, 6) == fuseDNAInstruction {
						selected = index
					}
				}
			case bitCount == 3+64+len(idcodes)-1+2:
				if selected < 0 {
					return errors.New("FUSE_DNA DR scan without selected device")
				}
				copy(tdo[3+selected:], uintToBits(dnaValues[selected], 64))
			}
			if err := writeAll(conn, packBits(tdo)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown XVC command prefix %q", command)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
