.PHONY: test vet build-windows clean

GO ?= go
GOFLAGS := -trimpath
LDFLAGS := -s -w
FYNE_CROSS ?= $(shell $(GO) env GOPATH)/bin/fyne-cross

RUNTIME := runtime/windows
PACKAGE := bin/fpga-dma-multi-tool-windows-amd64
ZIP := bin/FPGA-DMA-Multi-Tool-Windows-x64.zip
EXECUTABLE := FPGA-DMA-Multi-Tool.exe

test:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

build-windows:
	@test -x "$(FYNE_CROSS)" || { echo "fyne-cross is required" >&2; exit 1; }
	@command -v shasum >/dev/null 2>&1 || { echo "shasum is required" >&2; exit 1; }
	@command -v strings >/dev/null 2>&1 || { echo "strings is required" >&2; exit 1; }
	@command -v zip >/dev/null 2>&1 || { echo "zip is required" >&2; exit 1; }
	@test -f "$(RUNTIME)/cli-dma-speedtest-memflow-rs.exe" || { echo "Windows runtime files are missing" >&2; exit 1; }
	@echo "7375ff5e4e9584afc8b170ade5f2963f411e9f0ca9f35884fcb6f9f56a029f3b  $(RUNTIME)/cli-dma-speedtest-memflow-rs.exe" | shasum -a 256 -c -
	@echo "dc1817a34d018224ff023f4055bf9687a3a700500dce09bf37f775cc81e165e8  $(RUNTIME)/drivers/wch/CH341WDM.CAT" | shasum -a 256 -c -
	@echo "62cd5d5a6ce089086f4447ebad59aed783becece59ec42510eb862154278dcda  $(RUNTIME)/drivers/ftdi/ftdibus3.cat" | shasum -a 256 -c -
	@cd "$(RUNTIME)/openFPGALoader" && shasum -a 256 -c SHA256SUMS
	@if strings "$(RUNTIME)/openFPGALoader/openFPGALoader.exe" | grep -Fq "support for xvc-client was not enabled at compile time"; then \
		echo "openFPGALoader was built without XVC client support" >&2; exit 1; \
	fi
	@strings "$(RUNTIME)/openFPGALoader/openFPGALoader.exe" | \
		grep -Fq "detected %s version %s packet size" || \
		{ echo "openFPGALoader XVC client marker is missing" >&2; exit 1; }
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o "$(RUNTIME)/openFPGALoader/cygpath.exe" ./cmd/cygpath
	@cp windows-require-admin.manifest fpga-dma-multi-tool.exe.manifest
	@trap 'rm -f fpga-dma-multi-tool.exe.manifest' EXIT; \
	$(FYNE_CROSS) windows -arch=amd64 \
		-app-id com.sercanarga.fpgadmamultitool -app-version 2.0.0 \
		-name fpga-dma-multi-tool.exe -env GOTOOLCHAIN=auto .
	@rm -rf "$(PACKAGE)"
	@mkdir -p "$(PACKAGE)"
	@cp fyne-cross/bin/windows-amd64/fpga-dma-multi-tool.exe "$(PACKAGE)/$(EXECUTABLE)"
	@cp -R "$(RUNTIME)/." "$(PACKAGE)/"
	@rm -f "$(ZIP)"
	@cd "$(PACKAGE)" && zip -qr "../$(notdir $(ZIP))" .
	@echo "$(PACKAGE)"
	@echo "$(ZIP)"

clean:
	@rm -rf bin fyne-cross
	@rm -f "$(RUNTIME)/openFPGALoader/cygpath.exe"
