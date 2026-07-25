//go:build !windows

package main

import (
	"context"
	"fmt"
	"io"
)

func scanCH347(context.Context, options, io.Writer) ([]deviceResult, error) {
	return nil, fmt.Errorf("%w: direct CH347 access is available in the Windows executable", errCH347Unavailable)
}
