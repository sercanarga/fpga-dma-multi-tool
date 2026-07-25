//go:build !windows

package main

import (
	"context"
	"errors"
)

func prepareProgrammingTransport(
	_ context.Context,
	request programRequest,
) (programTransport, error) {
	if request.Cable == directCH347ProgrammingCable ||
		request.Cable == autoProgrammingCable ||
		request.Cable == rs232ProgrammingCable {
		return programTransport{}, errors.New("CH347 and RS232 programming are available in the Windows application")
	}
	return programTransport{Cable: request.Cable}, nil
}
