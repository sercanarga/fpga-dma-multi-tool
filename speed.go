package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const speedTestExecutable = "cli-dma-speedtest-memflow-rs.exe"

type speedTestRequest struct {
	Mode     string
	Duration int
	Sizes    []int
}

type speedTestReport struct {
	Connector   string          `json:"connector"`
	Mode        string          `json:"mode"`
	DurationSec int             `json:"duration_secs"`
	Sizes       []int           `json:"sizes"`
	Passes      []speedTestPass `json:"passes"`
}

type speedTestPass struct {
	Operation       string  `json:"op"`
	ChunkBytes      int     `json:"chunk_bytes"`
	AverageMiB      float64 `json:"avg_mib_s"`
	AverageOps      float64 `json:"avg_ops_s"`
	AverageLatency  float64 `json:"avg_latency_us"`
	Samples         int     `json:"samples"`
	TotalOperations int64   `json:"total_ops"`
	MeasuredSeconds float64 `json:"measured_secs"`
}

func (request speedTestRequest) validate() error {
	switch request.Mode {
	case "read", "write", "both":
	default:
		return fmt.Errorf("invalid speed test mode %q", request.Mode)
	}
	if request.Duration < 1 || request.Duration > 60 {
		return errors.New("speed test duration must be between 1 and 60 seconds")
	}
	if len(request.Sizes) == 0 {
		return errors.New("select at least one transfer size")
	}
	for _, size := range request.Sizes {
		if size <= 0 || size > 16*1024*1024 {
			return fmt.Errorf("invalid transfer size %d", size)
		}
	}
	return nil
}

func (request speedTestRequest) arguments(reportPath string) ([]string, error) {
	if err := request.validate(); err != nil {
		return nil, err
	}
	sizeValues := make([]string, len(request.Sizes))
	for index, size := range request.Sizes {
		sizeValues[index] = strconv.Itoa(size)
	}
	return []string{
		"--connector", "pcileech",
		"--device", "FPGA",
		"--duration", strconv.Itoa(request.Duration),
		"--mode", request.Mode,
		"--sizes", strings.Join(sizeValues, ","),
		"--output", reportPath,
		"--output-format", "json",
	}, nil
}

func speedTestPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate the application folder: %w", err)
	}
	path := filepath.Join(filepath.Dir(executable), speedTestExecutable)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("%s was not found beside the application", speedTestExecutable)
	}
	return path, nil
}

func runSpeedTest(
	ctx context.Context,
	request speedTestRequest,
	output io.Writer,
) (speedTestReport, error) {
	if err := validateSpeedTestEnvironment(); err != nil {
		return speedTestReport{}, err
	}
	runner, err := speedTestPath()
	if err != nil {
		return speedTestReport{}, err
	}
	tempDirectory, err := os.MkdirTemp("", "fpga-dma-multi-tool-speed-*")
	if err != nil {
		return speedTestReport{}, fmt.Errorf("cannot create report directory: %w", err)
	}
	defer os.RemoveAll(tempDirectory)

	reportPath := filepath.Join(tempDirectory, "report.json")
	arguments, err := request.arguments(reportPath)
	if err != nil {
		return speedTestReport{}, err
	}
	command := newSpeedTestCommand(ctx, runner, arguments)
	command.Stdin = strings.NewReader("\n")
	var diagnostics strings.Builder
	commandOutput := io.MultiWriter(output, &diagnostics)
	command.Stdout = commandOutput
	command.Stderr = commandOutput
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return speedTestReport{}, ctx.Err()
		}
		return speedTestReport{}, classifySpeedTestError(diagnostics.String(), err)
	}

	encoded, err := os.ReadFile(reportPath)
	if err != nil {
		return speedTestReport{}, fmt.Errorf("speed test did not create a report: %w", err)
	}
	var report speedTestReport
	if err := json.Unmarshal(encoded, &report); err != nil {
		return speedTestReport{}, fmt.Errorf("invalid speed test report: %w", err)
	}
	if len(report.Passes) == 0 {
		return speedTestReport{}, errors.New("speed test report contains no results")
	}
	return report, nil
}

func newSpeedTestCommand(ctx context.Context, runner string, arguments []string) *exec.Cmd {
	command := exec.CommandContext(ctx, runner, arguments...)
	// memflow discovers connector plugins relative to the process working
	// directory, not relative to the executable. Keep the child in the
	// packaged runtime directory so memflow_pcileech.dll is always found,
	// regardless of where the GUI was launched from.
	command.Dir = filepath.Dir(runner)
	configureChildProcess(command)
	return command
}

func classifySpeedTestError(diagnostics string, runErr error) error {
	message := strings.ToLower(diagnostics)
	switch {
	case strings.Contains(message, "inventory: plugin not found"):
		return errors.New("speed test connector plugin could not be loaded; reinstall or extract the complete application package")
	case strings.Contains(message, "connector: configuration error"):
		return errors.New("FT600/FT601 adapter or FTDI D3XX driver is unavailable; connect the DMA USB cable and check Drivers")
	case strings.Contains(message, "access is denied"),
		strings.Contains(message, "permission denied"):
		return errors.New("Windows denied access to the DMA adapter; run the application as administrator")
	default:
		return fmt.Errorf("speed test failed: %w", runErr)
	}
}

func formatTransferSize(bytes int) string {
	if bytes >= 1024*1024 && bytes%(1024*1024) == 0 {
		return fmt.Sprintf("%d MiB", bytes/(1024*1024))
	}
	if bytes >= 1024 && bytes%1024 == 0 {
		return fmt.Sprintf("%d KiB", bytes/1024)
	}
	return fmt.Sprintf("%d B", bytes)
}

func formatSpeedTestReport(report speedTestReport) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Connector: %s\nMode: %s\n\n", report.Connector, strings.ToUpper(report.Mode))
	for _, pass := range report.Passes {
		fmt.Fprintf(
			&output,
			"%s · %s\n%.1f MiB/s · %.0f ops/s · %.1f µs\n\n",
			strings.ToUpper(pass.Operation),
			formatTransferSize(pass.ChunkBytes),
			pass.AverageMiB,
			pass.AverageOps,
			pass.AverageLatency,
		)
	}
	return strings.TrimSpace(output.String())
}
