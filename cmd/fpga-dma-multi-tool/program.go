package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type programMode string

const (
	programSRAM  programMode = "sram"
	programFlash programMode = "flash"

	directCH347ProgrammingCable = "ch347-direct"
)

type programTransport struct {
	Cable       string
	Arguments   []string
	Description string
	Close       func() error
}

type programRequest struct {
	FilePath           string
	Mode               programMode
	OpenFPGALoaderPath string
	Cable              string
	FPGAPart           string
	ChainIndex         int
}

func (request programRequest) validate() error {
	info, err := os.Stat(request.FilePath)
	if err != nil {
		return fmt.Errorf("cannot open the selected file: %w", err)
	}
	if info.IsDir() {
		return errors.New("select a file, not a folder")
	}
	extension := strings.ToLower(filepath.Ext(request.FilePath))
	switch request.Mode {
	case programSRAM:
		if extension != ".bit" {
			return errors.New("SRAM mode requires a .bit file")
		}
	case programFlash:
		if extension != ".bin" {
			return errors.New("Flash mode requires a .bin file")
		}
	default:
		return errors.New("select a programming mode")
	}
	if strings.TrimSpace(request.FPGAPart) == "" {
		return errors.New("select the FPGA model")
	}
	if strings.TrimSpace(request.Cable) == "" {
		return errors.New("a programming cable is required")
	}
	if request.ChainIndex < 0 {
		return errors.New("the JTAG chain index cannot be negative")
	}
	return nil
}

func runProgramming(ctx context.Context, request programRequest, logWriter io.Writer) error {
	if err := request.validate(); err != nil {
		return err
	}
	executable, err := findOpenFPGALoader(strings.TrimSpace(request.OpenFPGALoaderPath))
	if err != nil {
		return err
	}
	transport, err := prepareProgrammingTransport(ctx, request)
	if err != nil {
		return err
	}
	if transport.Close != nil {
		defer transport.Close()
	}
	args := []string{
		"--cable", strings.TrimSpace(transport.Cable),
		"--fpga-part", strings.TrimSpace(request.FPGAPart),
	}
	args = append(args, transport.Arguments...)
	if request.ChainIndex > 0 {
		args = append(args, "--index-chain", strconv.Itoa(request.ChainIndex))
	}
	if request.Mode == programFlash {
		args = append(args, "-f")
	}
	args = append(args, request.FilePath)
	if transport.Description != "" {
		fmt.Fprintf(logWriter, "Transport: %s\n", transport.Description)
	}
	fmt.Fprintf(logWriter, "%s %s\n", executable, formatCommandArgs(args))
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = openFPGALoaderEnvironment(executable)
	command.Stdout = logWriter
	command.Stderr = logWriter
	configureChildProcess(command)
	if err := command.Run(); err != nil {
		return fmt.Errorf("programming failed: %w", err)
	}
	return nil
}

func runProgrammerCheck(ctx context.Context, logWriter io.Writer) error {
	executable, err := findOpenFPGALoader("")
	if err != nil {
		return err
	}
	transport, err := prepareProgrammingTransport(ctx, programRequest{
		Cable: directCH347ProgrammingCable,
	})
	if err != nil {
		return err
	}
	if transport.Close != nil {
		defer transport.Close()
	}
	args := []string{"--detect", "--cable", transport.Cable}
	args = append(args, transport.Arguments...)
	if transport.Description != "" {
		fmt.Fprintf(logWriter, "Transport: %s\n", transport.Description)
	}
	fmt.Fprintf(logWriter, "%s %s\n", executable, formatCommandArgs(args))
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = openFPGALoaderEnvironment(executable)
	command.Stdout = logWriter
	command.Stderr = logWriter
	configureChildProcess(command)
	if err := command.Run(); err != nil {
		return fmt.Errorf("programmer check failed: %w", err)
	}
	return nil
}

func findOpenFPGALoader(explicit string) (string, error) {
	if explicit != "" {
		absolute, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("cannot resolve openFPGALoader path: %w", err)
		}
		if info, err := os.Stat(absolute); err != nil || info.IsDir() {
			return "", fmt.Errorf("openFPGALoader was not found: %s", absolute)
		}
		return absolute, nil
	}
	if executable, err := bundledPath("openFPGALoader", "openFPGALoader.exe"); err == nil {
		return executable, nil
	}
	for _, name := range []string{"openFPGALoader", "openFPGALoader.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("the bundled programmer runtime is missing")
}

func openFPGALoaderRuntimeComplete(executable string) bool {
	if executable == "" {
		return false
	}
	directory := filepath.Dir(executable)
	required := []string{
		"openFPGALoader.exe",
		"cygpath.exe",
	}
	for _, name := range required {
		if info, err := os.Stat(filepath.Join(directory, name)); err != nil || info.IsDir() {
			return false
		}
	}
	for _, part := range []string{"15t", "35t", "50t", "75t", "100t", "200t"} {
		path := filepath.Join(directory, "data", "spiOverJtag_xc7a"+part+".bit.gz")
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return false
		}
	}
	return openFPGALoaderSupportsXVC(executable)
}

func openFPGALoaderSupportsXVC(executable string) bool {
	encoded, err := os.ReadFile(executable)
	if err != nil {
		return false
	}
	if bytes.Contains(encoded, []byte("support for xvc-client was not enabled at compile time")) {
		return false
	}
	for _, marker := range [][]byte{
		[]byte("xvc-client"),
		[]byte("detected %s version %s packet size"),
	} {
		if !bytes.Contains(encoded, marker) {
			return false
		}
	}
	return true
}

func bundledOpenFPGALoaderDataDirectory(executable string) string {
	if filepath.Base(executable) != "openFPGALoader.exe" {
		return ""
	}
	directory := filepath.Join(filepath.Dir(executable), "data")
	if info, err := os.Stat(directory); err == nil && info.IsDir() {
		return directory
	}
	return ""
}

func openFPGALoaderEnvironment(executable string) []string {
	environment := append([]string(nil), os.Environ()...)
	directory := filepath.Dir(executable)
	environment = setEnvironmentValue(
		environment,
		"PATH",
		directory+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	if sojDirectory := bundledOpenFPGALoaderDataDirectory(executable); sojDirectory != "" {
		environment = setEnvironmentValue(environment, "OPENFPGALOADER_SOJ_DIR", sojDirectory)
	}
	return environment
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	for index, entry := range environment {
		if separator := strings.IndexByte(entry, '='); separator >= 0 &&
			strings.EqualFold(entry[:separator], key) {
			environment[index] = prefix + value
			return environment
		}
	}
	return append(environment, prefix+value)
}

func formatCommandArgs(args []string) string {
	formatted := make([]string, len(args))
	for index, argument := range args {
		if strings.ContainsAny(argument, " \t\"") {
			formatted[index] = strconv.Quote(argument)
		} else {
			formatted[index] = argument
		}
	}
	return strings.Join(formatted, " ")
}
