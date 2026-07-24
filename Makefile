.PHONY: test vet build-windows clean

GO ?= go
GOFLAGS := -trimpath
LDFLAGS := -s -w
VERSION ?= 2.0.0
FYNE_CROSS ?= $(shell $(GO) env GOPATH)/bin/fyne-cross

BUILD_SOURCE := .build-source
PACKAGE := bin/fpga-dma-multi-tool-windows-amd64
ZIP := bin/FPGA-DMA-Multi-Tool-Windows-x64.zip
EXECUTABLE := FPGA-DMA-Multi-Tool.exe
OPENFPGALOADER_DATA := \
	spiOverJtag_xc7a15t.bit.gz \
	spiOverJtag_xc7a35t.bit.gz \
	spiOverJtag_xc7a50t.bit.gz \
	spiOverJtag_xc7a75t.bit.gz \
	spiOverJtag_xc7a100t.bit.gz \
	spiOverJtag_xc7a200t.bit.gz

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
	@echo "7375ff5e4e9584afc8b170ade5f2963f411e9f0ca9f35884fcb6f9f56a029f3b  cli-dma-speedtest-memflow-rs.exe" | shasum -a 256 -c -
	@echo "dc1817a34d018224ff023f4055bf9687a3a700500dce09bf37f775cc81e165e8  CH341WDM.CAT" | shasum -a 256 -c -
	@echo "62cd5d5a6ce089086f4447ebad59aed783becece59ec42510eb862154278dcda  ftdibus3.cat" | shasum -a 256 -c -
	@if strings openFPGALoader.exe | grep -Fq "support for xvc-client was not enabled at compile time"; then \
		echo "openFPGALoader was built without XVC client support" >&2; exit 1; \
	fi
	@strings openFPGALoader.exe | grep -Fq "detected %s version %s packet size" || \
		{ echo "openFPGALoader XVC client marker is missing" >&2; exit 1; }
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) \
		-tags cygpath -ldflags "$(LDFLAGS)" -o cygpath.exe cygpath.go
	@rm -rf "$(BUILD_SOURCE)"
	@mkdir -p "$(BUILD_SOURCE)"
	@cp *.go go.mod go.sum "$(BUILD_SOURCE)/"
	@sed 's/^var version = ".*"$$/var version = "$(VERSION)"/' \
		main.go > "$(BUILD_SOURCE)/main.go"
	@cp windows-require-admin.manifest "$(BUILD_SOURCE)/fpga-dma-multi-tool.exe.manifest"
	@$(FYNE_CROSS) windows -arch=amd64 \
		-app-id com.sercanarga.fpgadmamultitool -app-version "$(VERSION)" \
		-name fpga-dma-multi-tool.exe -env GOTOOLCHAIN=auto \
		-dir "$(CURDIR)/$(BUILD_SOURCE)" .
	@rm -f Icon.png
	@rm -rf "$(PACKAGE)"
	@mkdir -p \
		"$(PACKAGE)/drivers/wch" \
		"$(PACKAGE)/drivers/ftdi" \
		"$(PACKAGE)/openFPGALoader/data" \
		"$(PACKAGE)/openFPGALoader/licenses/gcc-libs" \
		"$(PACKAGE)/openFPGALoader/licenses/libwinpthread" \
		"$(PACKAGE)/openFPGALoader/licenses/zlib"
	@cp "$(BUILD_SOURCE)/fyne-cross/bin/windows-amd64/fpga-dma-multi-tool.exe" \
		"$(PACKAGE)/$(EXECUTABLE)"
	@cp CH347DLLA64.DLL FTD3XX.dll LICENSE-dma-speedtest-AGPL-3.0.txt \
		THIRD_PARTY_NOTICES.txt cli-dma-speedtest-memflow-rs.exe \
		memflow_pcileech.dll memflow_win32.dll "$(PACKAGE)/"
	@cp CH341DLL.DLL CH341DLLA64.DLL CH341M64.SYS CH341W64.SYS \
		CH341WDM.CAT CH341WDM.INF CH341WDM.SYS CH347DLL.DLL \
		CH347DLLA64.DLL "$(PACKAGE)/drivers/wch/"
	@cp ftdibus3.Inf ftdibus3.Sys ftdibus3.cat "$(PACKAGE)/drivers/ftdi/"
	@cp openFPGALoader.exe cygpath.exe "$(PACKAGE)/openFPGALoader/"
	@cp LICENSE-openFPGALoader.txt "$(PACKAGE)/openFPGALoader/LICENSE.openFPGALoader.txt"
	@cp openFPGALoader-SHA256SUMS "$(PACKAGE)/openFPGALoader/SHA256SUMS"
	@cp openFPGALoader-VERSION.txt "$(PACKAGE)/openFPGALoader/VERSION.txt"
	@for file in $(OPENFPGALOADER_DATA); do \
		cp "$$file" "$(PACKAGE)/openFPGALoader/data/$$file"; \
	done
	@cp LICENSE-gcc-libs-LGPL.txt "$(PACKAGE)/openFPGALoader/licenses/gcc-libs/COPYING.LIB"
	@cp LICENSE-gcc-runtime.txt "$(PACKAGE)/openFPGALoader/licenses/gcc-libs/COPYING.RUNTIME"
	@cp LICENSE-gcc-GPLv3.txt "$(PACKAGE)/openFPGALoader/licenses/gcc-libs/COPYING3"
	@cp NOTICE-gcc-libs.txt "$(PACKAGE)/openFPGALoader/licenses/gcc-libs/README"
	@cp LICENSE-libwinpthread.txt "$(PACKAGE)/openFPGALoader/licenses/libwinpthread/COPYING"
	@cp LICENSE-zlib.txt "$(PACKAGE)/openFPGALoader/licenses/zlib/LICENSE"
	@cd "$(PACKAGE)/openFPGALoader" && shasum -a 256 -c SHA256SUMS
	@rm -f "$(ZIP)"
	@cd "$(PACKAGE)" && zip -qr "../$(notdir $(ZIP))" .
	@rm -rf "$(BUILD_SOURCE)"
	@echo "$(PACKAGE)"
	@echo "$(ZIP)"

clean:
	@rm -rf bin fyne-cross "$(BUILD_SOURCE)"
	@rm -f cygpath.exe Icon.png
