//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

type ch347Library struct {
	dll        *syscall.DLL
	open       *syscall.Proc
	close      *syscall.Proc
	setTimeout *syscall.Proc
	jtagInit   *syscall.Proc
	resetTRST  *syscall.Proc
	writeData  *syscall.Proc
	readData   *syscall.Proc
}

type ch347Transport struct {
	ctx     context.Context
	library *ch347Library
	index   uint32
}

func scanCH347(ctx context.Context, opts options, diagnostics io.Writer) ([]deviceResult, error) {
	if opts.ch347Index < -1 || opts.ch347Index >= ch347MaxAdapters {
		return nil, fmt.Errorf("--ch347-index must be -1 through %d", ch347MaxAdapters-1)
	}
	library, path, err := loadCH347Library(opts.ch347DLL)
	if err != nil {
		return nil, err
	}
	defer library.dll.Release()
	if opts.verbose {
		fmt.Fprintf(diagnostics, "CH347 DLL: %s\n", path)
	}

	start, end := 0, ch347MaxAdapters
	if opts.ch347Index >= 0 {
		start, end = opts.ch347Index, opts.ch347Index+1
	}
	var results []deviceResult
	var opened int
	for adapterIndex := start; adapterIndex < end; adapterIndex++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !library.openDevice(uint32(adapterIndex)) {
			continue
		}
		opened++
		if opts.verbose {
			fmt.Fprintf(diagnostics, "CH347 adapter %d opened\n", adapterIndex)
		}
		scanErr := func() error {
			defer library.closeDevice(uint32(adapterIndex))
			if err := library.configure(uint32(adapterIndex)); err != nil {
				return err
			}
			transport := &ch347Transport{
				ctx:     ctx,
				library: library,
				index:   uint32(adapterIndex),
			}
			devices, err := scanDirectArtix(
				transport,
				opts.device,
				fmt.Sprintf("CH347[%d]", adapterIndex),
				"ch347-direct",
			)
			if err != nil {
				return err
			}
			for _, device := range devices {
				device.Index = len(results)
				results = append(results, device)
			}
			return nil
		}()
		if scanErr != nil {
			return nil, fmt.Errorf("CH347 adapter %d: %w", adapterIndex, scanErr)
		}
	}
	if opened == 0 {
		if opts.ch347Index >= 0 {
			return nil, fmt.Errorf("%w: adapter index %d was not found", errCH347Unavailable, opts.ch347Index)
		}
		return nil, fmt.Errorf("%w: no adapter was found at indexes 0-%d", errCH347Unavailable, ch347MaxAdapters-1)
	}
	if len(results) == 0 {
		return nil, errors.New("CH347 adapter opened, but no readable Artix-7 DNA was found")
	}
	return results, nil
}

func loadCH347Library(explicit string) (*ch347Library, string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else {
		if executable, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(executable), "CH347DLLA64.DLL"))
		}
		candidates = append(candidates, "CH347DLLA64.DLL")
	}

	var loadErrors []error
	for _, candidate := range candidates {
		dll, err := syscall.LoadDLL(candidate)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("%s: %w", candidate, err))
			continue
		}
		library, err := bindCH347Library(dll)
		if err != nil {
			dll.Release()
			loadErrors = append(loadErrors, fmt.Errorf("%s: %w", candidate, err))
			continue
		}
		return library, candidate, nil
	}
	return nil, "", fmt.Errorf(
		"%w: CH347DLLA64.DLL could not be loaded (%v)",
		errCH347Unavailable,
		errors.Join(loadErrors...),
	)
}

func bindCH347Library(dll *syscall.DLL) (*ch347Library, error) {
	find := func(name string) (*syscall.Proc, error) {
		procedure, err := dll.FindProc(name)
		if err != nil {
			return nil, fmt.Errorf("required function %s is missing: %w", name, err)
		}
		return procedure, nil
	}
	open, err := find("CH347OpenDevice")
	if err != nil {
		return nil, err
	}
	closeDevice, err := find("CH347CloseDevice")
	if err != nil {
		return nil, err
	}
	setTimeout, err := find("CH347SetTimeout")
	if err != nil {
		return nil, err
	}
	jtagInit, err := find("CH347Jtag_INIT")
	if err != nil {
		return nil, err
	}
	resetTRST, err := find("CH347Jtag_ResetTrst")
	if err != nil {
		return nil, err
	}
	writeData, err := find("CH347WriteData")
	if err != nil {
		return nil, err
	}
	readData, err := find("CH347ReadData")
	if err != nil {
		return nil, err
	}
	return &ch347Library{
		dll:        dll,
		open:       open,
		close:      closeDevice,
		setTimeout: setTimeout,
		jtagInit:   jtagInit,
		resetTRST:  resetTRST,
		writeData:  writeData,
		readData:   readData,
	}, nil
}

func (library *ch347Library) openDevice(index uint32) bool {
	result, _, _ := library.open.Call(uintptr(index))
	// The vendor header declares CH347OpenDevice as returning a 32-bit int,
	// even in CH347DLLA64.DLL. Compare the low 32 bits with -1 instead of
	// assuming a pointer-width INVALID_HANDLE_VALUE.
	return uint32(result) != ^uint32(0)
}

func (library *ch347Library) closeDevice(index uint32) {
	library.close.Call(uintptr(index))
}

func (library *ch347Library) configure(index uint32) error {
	ok, _, callErr := library.setTimeout.Call(uintptr(index), 1500, 1500)
	if ok == 0 {
		return fmt.Errorf("CH347SetTimeout failed: %v", callErr)
	}
	ok, _, callErr = library.jtagInit.Call(uintptr(index), ch347JTAGClockIndex)
	if ok == 0 {
		return fmt.Errorf("CH347Jtag_INIT failed: %v", callErr)
	}
	ok, _, callErr = library.resetTRST.Call(uintptr(index), 1)
	if ok == 0 {
		return fmt.Errorf("CH347Jtag_ResetTrst failed: %v", callErr)
	}
	time.Sleep(250 * time.Microsecond)
	return nil
}

func (transport *ch347Transport) shift(tms, tdi []byte) ([]byte, error) {
	if len(tms) != len(tdi) {
		return nil, errors.New("internal CH347 TMS/TDI length mismatch")
	}
	tdo := make([]byte, len(tms))
	for offset := 0; offset < len(tms); {
		if err := transport.ctx.Err(); err != nil {
			return nil, err
		}
		count := min(ch347MaxBitsPerScan, len(tms)-offset)
		command, err := buildCH347BitCommand(tms[offset:offset+count], tdi[offset:offset+count])
		if err != nil {
			return nil, err
		}
		if err := transport.library.write(transport.index, command); err != nil {
			return nil, err
		}
		response, err := transport.library.readResponse(transport.index, count)
		if err != nil {
			return nil, err
		}
		bits, err := decodeCH347BitResponse(response, count)
		if err != nil {
			return nil, err
		}
		copy(tdo[offset:], bits)
		offset += count
		time.Sleep(250 * time.Microsecond)
	}
	return tdo, nil
}

func (library *ch347Library) write(index uint32, data []byte) error {
	length := uint32(len(data))
	ok, _, callErr := library.writeData.Call(
		uintptr(index),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(unsafe.Pointer(&length)),
	)
	runtime.KeepAlive(data)
	if ok == 0 {
		return fmt.Errorf("CH347WriteData failed: %v", callErr)
	}
	if length != uint32(len(data)) {
		return fmt.Errorf("CH347WriteData wrote %d of %d bytes", length, len(data))
	}
	return nil
}

func (library *ch347Library) readResponse(index uint32, payloadLength int) ([]byte, error) {
	expected := 3 + payloadLength
	response := make([]byte, 0, expected)
	for len(response) < expected {
		buffer := make([]byte, 512)
		length := uint32(len(buffer))
		ok, _, callErr := library.readData.Call(
			uintptr(index),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&length)),
		)
		runtime.KeepAlive(buffer)
		if ok == 0 {
			return nil, fmt.Errorf("CH347ReadData failed: %v", callErr)
		}
		if length == 0 {
			return nil, errors.New("CH347ReadData returned an empty response")
		}
		response = append(response, buffer[:length]...)
		if len(response) >= 3 {
			reported := int(binary.LittleEndian.Uint16(response[1:3]))
			if reported != payloadLength {
				return nil, fmt.Errorf(
					"CH347 response payload length is %d, expected %d",
					reported, payloadLength,
				)
			}
			expected = 3 + reported
		}
	}
	if len(response) != expected {
		return nil, fmt.Errorf("CH347 response length is %d, expected %d", len(response), expected)
	}
	return response, nil
}
