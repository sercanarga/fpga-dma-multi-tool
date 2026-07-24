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
	if request.Cable == directCH347ProgrammingCable {
		return programTransport{}, errors.New("direct CH347 programming is available in the Windows application")
	}
	return programTransport{Cable: request.Cable}, nil
}
