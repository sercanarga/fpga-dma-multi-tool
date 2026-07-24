//go:build !windows

package main

import "os/exec"

func configureChildProcess(*exec.Cmd) {}

func validateSpeedTestEnvironment() error { return nil }

func inspectFTDID3XXAdapter(string) (bool, error) { return false, nil }
