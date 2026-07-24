package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgramRequestSeparatesSRAMAndFlashFormats(t *testing.T) {
	tempDir := t.TempDir()
	bitPath := filepath.Join(tempDir, "firmware.bit")
	binPath := filepath.Join(tempDir, "firmware.bin")
	for _, path := range []string{bitPath, binPath} {
		if err := os.WriteFile(path, []byte{0xaa}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	base := programRequest{
		Cable: directCH347ProgrammingCable, FPGAPart: "xc7a100t", ChainIndex: 0,
	}
	base.Mode, base.FilePath = programSRAM, bitPath
	if err := base.validate(); err != nil {
		t.Fatalf("valid SRAM request rejected: %v", err)
	}
	base.FilePath = binPath
	if err := base.validate(); err == nil || !strings.Contains(err.Error(), ".bit") {
		t.Fatalf("SRAM accepted .bin file: %v", err)
	}

	base.Mode, base.FilePath = programFlash, binPath
	if err := base.validate(); err != nil {
		t.Fatalf("valid flash request rejected: %v", err)
	}
	base.FilePath = bitPath
	if err := base.validate(); err == nil || !strings.Contains(err.Error(), ".bin") {
		t.Fatalf("flash accepted .bit file: %v", err)
	}
}

func TestProgramRequestRequiresExactProgrammingMetadata(t *testing.T) {
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "firmware.bin")
	if err := os.WriteFile(binPath, []byte{0xaa}, 0o600); err != nil {
		t.Fatal(err)
	}

	request := programRequest{
		FilePath: binPath, Mode: programFlash, Cable: directCH347ProgrammingCable,
	}
	if err := request.validate(); err == nil || !strings.Contains(err.Error(), "FPGA model") {
		t.Fatalf("missing FPGA model accepted: %v", err)
	}
	request.FPGAPart = "xc7a100t"
	request.ChainIndex = -1
	if err := request.validate(); err == nil || !strings.Contains(err.Error(), "chain index") {
		t.Fatalf("negative chain index accepted: %v", err)
	}
}

func TestOpenFPGALoaderFlashFlagIsExplicit(t *testing.T) {
	request := programRequest{Mode: programFlash}
	args := []string{"--cable", directCH347ProgrammingCable, "--fpga-part", "xc7a100t"}
	if request.Mode == programFlash {
		args = append(args, "-f")
	}
	args = append(args, `C:\firmware files\card.bin`)
	formatted := formatCommandArgs(args)
	if !strings.Contains(formatted, " -f ") {
		t.Fatalf("flash flag missing from %q", formatted)
	}
	if !strings.Contains(formatted, `"C:\\firmware files\\card.bin"`) {
		t.Fatalf("path with spaces was not quoted: %q", formatted)
	}
}

func TestBundledOpenFPGALoaderEnvironmentSupportsSpaces(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "FPGA Multi Tool", "openFPGALoader")
	if err := os.MkdirAll(filepath.Join(directory, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "openFPGALoader.exe")
	if err := os.WriteFile(executable, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := openFPGALoaderEnvironment(executable)
	values := make(map[string]string)
	for _, entry := range environment {
		if separator := strings.IndexByte(entry, '='); separator >= 0 {
			values[strings.ToUpper(entry[:separator])] = entry[separator+1:]
		}
	}
	if got := values["OPENFPGALOADER_SOJ_DIR"]; got != filepath.Join(directory, "data") {
		t.Fatalf("OPENFPGALOADER_SOJ_DIR = %q", got)
	}
	pathValue := values["PATH"]
	if !strings.HasPrefix(pathValue, directory+string(os.PathListSeparator)) {
		t.Fatalf("PATH does not start with bundled directory: %q", pathValue)
	}
}

func TestOpenFPGALoaderRuntimeCompleteRequiresEveryBridge(t *testing.T) {
	directory := t.TempDir()
	required := []string{
		"openFPGALoader.exe",
		"cygpath.exe",
		"libftdi1.dll",
		"libgcc_s_seh-1.dll",
		"libstdc++-6.dll",
		"libusb-1.0.dll",
		"libwinpthread-1.dll",
		"zlib1.dll",
	}
	for _, name := range required {
		content := []byte("test")
		if name == "openFPGALoader.exe" {
			content = []byte("xvc-client detected %s version %s packet size")
		}
		if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dataDirectory := filepath.Join(directory, "data")
	if err := os.Mkdir(dataDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	parts := []string{"15t", "35t", "50t", "75t", "100t", "200t"}
	for _, part := range parts {
		path := filepath.Join(dataDirectory, "spiOverJtag_xc7a"+part+".bit.gz")
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(directory, "openFPGALoader.exe")
	if !openFPGALoaderRuntimeComplete(executable) {
		t.Fatal("complete runtime was reported as incomplete")
	}
	if err := os.Remove(filepath.Join(dataDirectory, "spiOverJtag_xc7a75t.bit.gz")); err != nil {
		t.Fatal(err)
	}
	if openFPGALoaderRuntimeComplete(executable) {
		t.Fatal("runtime without the 75T bridge was reported as complete")
	}
}

func TestOpenFPGALoaderRuntimeRejectsDisabledXVCClient(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "openFPGALoader.exe")
	message := []byte("Jtag: support for xvc-client was not enabled at compile time")
	if err := os.WriteFile(executable, message, 0o600); err != nil {
		t.Fatal(err)
	}
	if openFPGALoaderSupportsXVC(executable) {
		t.Fatal("binary with disabled XVC client was accepted")
	}
}

func TestOpenFPGALoaderRuntimeRejectsUnrelatedExecutable(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "openFPGALoader.exe")
	if err := os.WriteFile(executable, []byte("not a programmer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if openFPGALoaderSupportsXVC(executable) {
		t.Fatal("unrelated executable was accepted as an XVC programmer")
	}
}
