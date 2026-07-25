package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

type xvcLoopbackTransport struct{}

func (xvcLoopbackTransport) shift(tms, tdi []byte) ([]byte, error) {
	tdo := append([]byte(nil), tdi...)
	for index := range tdo {
		tdo[index] ^= tms[index]
	}
	return tdo, nil
}

func TestLocalXVCBridgeProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bridge, err := startXVCBridge(ctx, xvcLoopbackTransport{})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()

	for session := 0; session < 2; session++ {
		connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", bridge.Address())
		if err != nil {
			t.Fatal(err)
		}
		client := &xvcClient{conn: connection}

		info, err := client.getInfo()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(info, "xvcServer_v1.0:") {
			t.Fatalf("unexpected XVC info %q", info)
		}
		if period, err := client.setTCK(2133); err != nil || period != 2133 {
			t.Fatalf("setTCK() = %d, %v", period, err)
		}
		tms := []byte{0, 1, 0, 1, 1, 0, 0, 1, 0}
		tdi := []byte{1, 1, 0, 0, 1, 1, 0, 0, 1}
		tdo, err := client.shift(tms, tdi)
		if err != nil {
			t.Fatal(err)
		}
		for index := range tdo {
			if want := tms[index] ^ tdi[index]; tdo[index] != want {
				t.Fatalf("session %d TDO[%d] = %d, want %d", session, index, tdo[index], want)
			}
		}
		_ = connection.Close()
	}
}
