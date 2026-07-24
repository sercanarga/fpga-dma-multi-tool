package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSpeedTestArguments(t *testing.T) {
	request := speedTestRequest{
		Mode: "both", Duration: 5, Sizes: []int{4096, 8192, 32768},
	}
	got, err := request.arguments(`C:\Temp\report.json`)
	if err != nil {
		t.Fatalf("arguments() error = %v", err)
	}
	want := []string{
		"--connector", "pcileech",
		"--device", "FPGA",
		"--duration", "5",
		"--mode", "both",
		"--sizes", "4096,8192,32768",
		"--output", `C:\Temp\report.json`,
		"--output-format", "json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments() = %#v, want %#v", got, want)
	}
}

func TestClassifySpeedTestError(t *testing.T) {
	err := classifySpeedTestError(
		"Error: PCILeech connector error: connector: configuration error",
		errors.New("exit status 1"),
	)
	if !strings.Contains(err.Error(), "FT600/FT601") ||
		!strings.Contains(err.Error(), "check Drivers") {
		t.Fatalf("configuration error = %q", err)
	}

	err = classifySpeedTestError("unexpected", errors.New("exit status 2"))
	if !strings.Contains(err.Error(), "exit status 2") {
		t.Fatalf("fallback error = %q", err)
	}
}

func TestClassifySpeedTestErrorExplainsMissingPlugin(t *testing.T) {
	err := classifySpeedTestError(
		"Error: PCILeech connector error: inventory: plugin not found",
		errors.New("exit status 1"),
	)
	if err == nil || !strings.Contains(err.Error(), "complete application package") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewSpeedTestCommandUsesPackagedRuntimeDirectory(t *testing.T) {
	runner := filepath.Join(t.TempDir(), "runtime", speedTestExecutable)
	command := newSpeedTestCommand(context.Background(), runner, []string{"--version"})
	if got, want := command.Dir, filepath.Dir(runner); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
}

func TestSpeedTestRequestValidation(t *testing.T) {
	tests := []speedTestRequest{
		{Mode: "invalid", Duration: 5, Sizes: []int{4096}},
		{Mode: "read", Duration: 0, Sizes: []int{4096}},
		{Mode: "read", Duration: 5},
		{Mode: "read", Duration: 5, Sizes: []int{0}},
	}
	for _, request := range tests {
		if err := request.validate(); err == nil {
			t.Errorf("validate(%+v) succeeded, want error", request)
		}
	}
}

func TestFormatTransferSize(t *testing.T) {
	for input, want := range map[int]string{
		512:     "512 B",
		4096:    "4 KiB",
		1 << 20: "1 MiB",
	} {
		if got := formatTransferSize(input); got != want {
			t.Errorf("formatTransferSize(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatSpeedTestReport(t *testing.T) {
	report := speedTestReport{
		Connector: "pcileech",
		Mode:      "read",
		Passes: []speedTestPass{{
			Operation:      "read",
			ChunkBytes:     4096,
			AverageMiB:     125.5,
			AverageOps:     32128,
			AverageLatency: 3.2,
		}},
	}
	output := formatSpeedTestReport(report)
	for _, expected := range []string{
		"Connector: pcileech",
		"Mode: READ",
		"READ · 4 KiB",
		"125.5 MiB/s · 32128 ops/s · 3.2 µs",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("formatted report %q does not contain %q", output, expected)
		}
	}
}
