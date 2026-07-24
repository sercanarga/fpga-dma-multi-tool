//go:build !windows

package main

import (
	"context"
	"errors"
)

var errDeviceHistoryUnavailable = errors.New("device history management is available in the Windows build")

func scanDeviceHistory(context.Context) ([]deviceHistoryEntry, error) {
	return nil, errDeviceHistoryUnavailable
}

func removeDeviceHistory(context.Context, deviceHistoryEntry) (deviceHistoryCleanupResult, error) {
	return deviceHistoryCleanupResult{}, errDeviceHistoryUnavailable
}

func rescanWindowsDevices(context.Context) error {
	return errDeviceHistoryUnavailable
}
