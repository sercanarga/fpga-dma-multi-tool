//go:build !windows

package main

import (
	"context"
	"errors"
	"io"
)

func scanRS232(context.Context, options, io.Writer) ([]deviceResult, error) {
	return nil, errors.New("RS232 FPGA detection is available in the Windows application")
}
