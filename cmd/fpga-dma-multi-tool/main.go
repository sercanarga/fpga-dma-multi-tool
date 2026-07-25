package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "2.0.0"

type options struct {
	backend    string
	xvcAddress string
	device     int
	ch347DLL   string
	ch347Index int
	timeout    time.Duration
	json       bool
	verbose    bool
}

type deviceResult struct {
	Index        int      `json:"index"`
	Target       string   `json:"target,omitempty"`
	Name         string   `json:"name"`
	Part         string   `json:"part,omitempty"`
	IDCode       string   `json:"idcode,omitempty"`
	FuseDNA      string   `json:"fuse_dna"`
	DeviceDNA    string   `json:"device_dna,omitempty"`
	BoardMatches []string `json:"board_matches,omitempty"`
	Backend      string   `json:"backend"`
}

type scanResult struct {
	ToolVersion string         `json:"tool_version"`
	Devices     []deviceResult `json:"devices"`
}

func main() {
	if len(os.Args) == 1 {
		launchGUI()
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--gui" {
		launchGUI()
		return
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var opts options
	flags := flag.NewFlagSet("fpga-dma-multi-tool", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.backend, "backend", "auto", "backend: auto, ch347, rs232, or xvc")
	flags.StringVar(&opts.xvcAddress, "xvc", "", "XVC server address for direct JTAG, for example 127.0.0.1:2542")
	flags.IntVar(&opts.device, "device", -1, "JTAG chain index for a direct backend; -1 reads all")
	flags.StringVar(&opts.ch347DLL, "ch347-dll", "", "optional path to CH347DLLA64.DLL")
	flags.IntVar(&opts.ch347Index, "ch347-index", -1, "CH347 adapter index; -1 scans every adapter")
	flags.DurationVar(&opts.timeout, "timeout", 45*time.Second, "overall scan timeout")
	flags.BoolVar(&opts.json, "json", false, "emit machine-readable JSON")
	flags.BoolVar(&opts.verbose, "verbose", false, "show backend diagnostics")
	showVersion := flags.Bool("version", false, "show version")
	programmerCheck := flags.Bool("programmer-check", false, "check the selected FPGA programmer")
	programFile := flags.String("program-file", "", "program a .bit or .bin file")
	programmer := flags.String("programmer", "auto", "programmer: auto, ch347, or rs232")
	programModeFlag := flags.String("program-mode", "sram", "program mode: sram or flash")
	programPart := flags.String("fpga-part", "xc7a100t", "FPGA model used for programming")
	programChainIndex := flags.Int("chain-index", 0, "zero-based JTAG chain index used for programming")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "FPGA DMA Multi Tool reads immutable FUSE_DNA values over JTAG.")
		fmt.Fprintln(stderr, "Run without options (or with --gui) to open the graphical interface.")
		fmt.Fprintln(stderr, "Usage: FPGA-DMA-Multi-Tool.exe [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if opts.timeout <= 0 {
		fmt.Fprintln(stderr, "--timeout must be greater than zero")
		return 2
	}
	if *programmerCheck && strings.TrimSpace(*programFile) != "" {
		fmt.Fprintln(stderr, "--programmer-check cannot be combined with --program-file")
		return 2
	}
	programmingCable, err := programmingCableFromName(*programmer)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	if *programmerCheck {
		if err := runProgrammerCheck(ctx, programmingCable, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Bundled programmer is ready.")
		return 0
	}
	if strings.TrimSpace(*programFile) != "" {
		mode := programMode(strings.ToLower(strings.TrimSpace(*programModeFlag)))
		request := programRequest{
			FilePath:   strings.TrimSpace(*programFile),
			Mode:       mode,
			Cable:      programmingCable,
			FPGAPart:   strings.ToLower(strings.TrimSpace(*programPart)),
			ChainIndex: *programChainIndex,
		}
		if err := runProgramming(ctx, request, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "FPGA programming completed successfully.")
		return 0
	}

	backend, err := selectBackend(opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	var devices []deviceResult
	switch backend {
	case "auto":
		devices, err = scanAutomatic(ctx, opts, stderr)
	case "ch347":
		devices, err = scanCH347(ctx, opts, stderr)
	case "rs232":
		devices, err = scanRS232(ctx, opts, stderr)
	case "xvc":
		devices, err = scanXVC(ctx, opts, stderr)
	default:
		err = fmt.Errorf("internal error: unsupported backend %q", backend)
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("scan timed out after %s", opts.timeout)
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(devices) == 0 {
		fmt.Fprintln(stderr, "error: no FPGA DNA values were found")
		return 1
	}

	result := scanResult{ToolVersion: version, Devices: devices}
	if opts.json {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "error: write JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printHuman(stdout, result)
	return 0
}

func selectBackend(opts options) (string, error) {
	backend := strings.ToLower(strings.TrimSpace(opts.backend))
	switch backend {
	case "auto":
		if opts.xvcAddress != "" {
			return "xvc", nil
		}
		return "auto", nil
	case "ch347", "rs232":
		if opts.xvcAddress != "" {
			return "", fmt.Errorf("--xvc cannot be combined with --backend %s", backend)
		}
		return backend, nil
	case "xvc":
		if opts.xvcAddress == "" {
			return "", errors.New("--backend xvc requires --xvc host:port")
		}
		return backend, nil
	default:
		return "", fmt.Errorf("invalid --backend %q (expected auto, ch347, rs232, or xvc)", opts.backend)
	}
}

func printHuman(w io.Writer, result scanResult) {
	fmt.Fprintf(w, "FPGA DMA Multi Tool %s\n", result.ToolVersion)
	fmt.Fprintf(w, "Found %d FPGA device(s)\n\n", len(result.Devices))
	for _, device := range result.Devices {
		fmt.Fprintf(w, "[%d] %s\n", device.Index, device.Name)
		if device.Target != "" {
			fmt.Fprintf(w, "    Target:      %s\n", device.Target)
		}
		if device.Part != "" {
			fmt.Fprintf(w, "    Part:        %s\n", device.Part)
		}
		if device.IDCode != "" {
			fmt.Fprintf(w, "    IDCODE:      %s\n", device.IDCode)
		}
		fmt.Fprintf(w, "    FUSE_DNA:    %s (64-bit immutable identifier)\n", device.FuseDNA)
		if device.DeviceDNA != "" {
			fmt.Fprintf(w, "    Device DNA:  %s (FUSE_DNA[63:7])\n", device.DeviceDNA)
		}
		if len(device.BoardMatches) != 0 {
			fmt.Fprintf(w, "    Boards:      %s\n", strings.Join(device.BoardMatches, ", "))
		}
		fmt.Fprintf(w, "    Backend:     %s\n\n", device.Backend)
	}
}
