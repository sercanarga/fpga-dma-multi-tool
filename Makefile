.PHONY: test vet build-windows clean

GO ?= go
GOFLAGS := -trimpath
LDFLAGS := -s -w
VERSION ?= 2.0.0
FYNE_CROSS ?= $(shell $(GO) env GOPATH)/bin/fyne-cross

APP_SOURCE := cmd/fpga-dma-multi-tool
WINDOWS_PACKAGE := packaging/windows
WINDOWS_RUNTIME := $(WINDOWS_PACKAGE)/runtime
BUILD_SOURCE := .build-source
BUILD_CYGPATH := .build-cygpath.exe
PACKAGE := bin/fpga-dma-multi-tool-windows-amd64
ZIP := bin/FPGA-DMA-Multi-Tool-Windows-x64.zip
EXECUTABLE := FPGA-DMA-Multi-Tool.exe

test:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

build-windows:
	@test -x "$(FYNE_CROSS)" || { echo "fyne-cross is required" >&2; exit 1; }
	@case "$(VERSION)" in \
		""|*[!0-9A-Za-z.-]*) echo "VERSION contains invalid characters" >&2; exit 1;; \
	esac
	@command -v shasum >/dev/null 2>&1 || { echo "shasum is required" >&2; exit 1; }
	@command -v strings >/dev/null 2>&1 || { echo "strings is required" >&2; exit 1; }
	@command -v zip >/dev/null 2>&1 || { echo "zip is required" >&2; exit 1; }
	@echo "7375ff5e4e9584afc8b170ade5f2963f411e9f0ca9f35884fcb6f9f56a029f3b  $(WINDOWS_RUNTIME)/cli-dma-speedtest-memflow-rs.exe" | shasum -a 256 -c -
	@echo "dc1817a34d018224ff023f4055bf9687a3a700500dce09bf37f775cc81e165e8  $(WINDOWS_RUNTIME)/drivers/wch/CH341WDM.CAT" | shasum -a 256 -c -
	@echo "62cd5d5a6ce089086f4447ebad59aed783becece59ec42510eb862154278dcda  $(WINDOWS_RUNTIME)/drivers/ftdi/ftdibus3.cat" | shasum -a 256 -c -
	@cd "$(WINDOWS_RUNTIME)/openFPGALoader" && shasum -a 256 -c SHA256SUMS
	@if strings "$(WINDOWS_RUNTIME)/openFPGALoader/openFPGALoader.exe" | grep -Fq "support for xvc-client was not enabled at compile time"; then \
		echo "openFPGALoader was built without XVC client support" >&2; exit 1; \
	fi
	@strings "$(WINDOWS_RUNTIME)/openFPGALoader/openFPGALoader.exe" | \
		grep -Fq "detected %s version %s packet size" || \
		{ echo "openFPGALoader XVC client marker is missing" >&2; exit 1; }
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) \
		-ldflags "$(LDFLAGS)" -o "$(BUILD_CYGPATH)" ./cmd/cygpath
	@rm -rf "$(BUILD_SOURCE)"
	@mkdir -p "$(BUILD_SOURCE)"
	@cp "$(APP_SOURCE)"/*.go go.mod go.sum "$(BUILD_SOURCE)/"
	@sed 's/^var version = ".*"$$/var version = "$(VERSION)"/' \
		"$(APP_SOURCE)/main.go" > "$(BUILD_SOURCE)/main.go"
	@cp "$(WINDOWS_PACKAGE)/windows-require-admin.manifest" \
		"$(BUILD_SOURCE)/fpga-dma-multi-tool.exe.manifest"
	@$(FYNE_CROSS) windows -arch=amd64 \
		-app-id com.sercanarga.fpgadmamultitool -app-version "$(VERSION)" \
		-name fpga-dma-multi-tool.exe -env GOTOOLCHAIN=auto \
		-dir "$(CURDIR)/$(BUILD_SOURCE)" .
	@rm -f Icon.png
	@rm -rf "$(PACKAGE)"
	@mkdir -p "$(PACKAGE)"
	@cp "$(BUILD_SOURCE)/fyne-cross/bin/windows-amd64/fpga-dma-multi-tool.exe" \
		"$(PACKAGE)/$(EXECUTABLE)"
	@cp -R "$(WINDOWS_RUNTIME)/." "$(PACKAGE)/"
	@cp "$(BUILD_CYGPATH)" "$(PACKAGE)/openFPGALoader/cygpath.exe"
	@cd "$(PACKAGE)/openFPGALoader" && shasum -a 256 -c SHA256SUMS
	@rm -f "$(ZIP)"
	@cd "$(PACKAGE)" && zip -qr "../$(notdir $(ZIP))" .
	@rm -rf "$(BUILD_SOURCE)"
	@rm -f "$(BUILD_CYGPATH)"
	@echo "$(PACKAGE)"
	@echo "$(ZIP)"

clean:
	@rm -rf bin fyne-cross "$(BUILD_SOURCE)"
	@rm -f "$(BUILD_CYGPATH)" cygpath.exe fpga-dma-multi-tool.exe Icon.png
