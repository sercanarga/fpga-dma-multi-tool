# Windows runtime

`packaging/windows/runtime` is copied unchanged into the Windows release
directory. Files at its root must remain beside `FPGA-DMA-Multi-Tool.exe`
because the application and external tools load them at runtime.

## Directory layout

- `drivers/wch` contains the signed CH347 driver and vendor DLLs.
- `drivers/ftdi` contains the signed FTDI D3XX package for FT600/FT601.
- `drivers/rs232` contains Zadig/libwdi, its configuration, licenses, and source.
- `openFPGALoader` contains the XVC executable, the separate FTDI/libusb
  executable with the RS DMA cable profile, their DLLs, bridge bitstreams,
  hashes, and third-party licenses.
- Root DLLs and the DMA benchmark executable are runtime dependencies.

Do not normalize the capitalization or filenames inside signed vendor driver
packages. INF and catalog metadata can refer to those exact names.

## Verification

Run:

```sh
make verify-runtime
```

The target verifies pinned SHA-256 hashes and real XVC, FTDI, and RS DMA feature
markers. `make build-windows` runs this check automatically before packaging.

Third-party provenance and redistribution notes are recorded in
`packaging/windows/runtime/THIRD_PARTY_NOTICES.txt`.
