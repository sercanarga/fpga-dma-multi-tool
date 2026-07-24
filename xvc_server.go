package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const xvcBridgeMaxBits = 1 << 20

type xvcBridge struct {
	listener   net.Listener
	transport  jtagTransport
	closeOnce  sync.Once
	mutex      sync.Mutex
	connection net.Conn
}

func startXVCBridge(ctx context.Context, transport jtagTransport) (*xvcBridge, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local XVC bridge: %w", err)
	}
	bridge := &xvcBridge{listener: listener, transport: transport}
	go func() {
		<-ctx.Done()
		_ = bridge.Close()
	}()
	go bridge.serve(ctx)
	return bridge, nil
}

func (bridge *xvcBridge) Address() string {
	return bridge.listener.Addr().String()
}

func (bridge *xvcBridge) Close() error {
	var err error
	bridge.closeOnce.Do(func() {
		err = bridge.listener.Close()
		bridge.mutex.Lock()
		if bridge.connection != nil {
			_ = bridge.connection.Close()
		}
		bridge.mutex.Unlock()
	})
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (bridge *xvcBridge) serve(ctx context.Context) {
	for {
		connection, err := bridge.listener.Accept()
		if err != nil {
			return
		}
		bridge.mutex.Lock()
		bridge.connection = connection
		bridge.mutex.Unlock()
		_ = serveXVCConnection(ctx, connection, bridge.transport)
		_ = connection.Close()
		bridge.mutex.Lock()
		if bridge.connection == connection {
			bridge.connection = nil
		}
		bridge.mutex.Unlock()
		if err := ctx.Err(); err != nil {
			return
		}
	}
}

func serveXVCConnection(ctx context.Context, connection net.Conn, transport jtagTransport) error {
	reader := bufio.NewReader(connection)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		command, err := reader.ReadString(':')
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read XVC command: %w", err)
		}
		switch command {
		case "getinfo:":
			if err := writeAll(connection, []byte(fmt.Sprintf("xvcServer_v1.0:%d\n", xvcBridgeMaxBits))); err != nil {
				return err
			}
		case "settck:":
			var period [4]byte
			if _, err := io.ReadFull(reader, period[:]); err != nil {
				return fmt.Errorf("read XVC clock period: %w", err)
			}
			if err := writeAll(connection, period[:]); err != nil {
				return err
			}
		case "shift:":
			var countBuffer [4]byte
			if _, err := io.ReadFull(reader, countBuffer[:]); err != nil {
				return fmt.Errorf("read XVC shift length: %w", err)
			}
			bitCount := int(binary.LittleEndian.Uint32(countBuffer[:]))
			if bitCount <= 0 || bitCount > xvcBridgeMaxBits {
				return fmt.Errorf("XVC shift length must be 1-%d bits, got %d", xvcBridgeMaxBits, bitCount)
			}
			byteCount := (bitCount + 7) / 8
			payload := make([]byte, 2*byteCount)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return fmt.Errorf("read XVC shift payload: %w", err)
			}
			tms := unpackBits(payload[:byteCount], bitCount)
			tdi := unpackBits(payload[byteCount:], bitCount)
			tdo, err := transport.shift(tms, tdi)
			if err != nil {
				return fmt.Errorf("execute XVC shift: %w", err)
			}
			if err := writeAll(connection, packBits(tdo)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported XVC command %q", command)
		}
	}
}
