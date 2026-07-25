RS232 writer driver setup
=========================

This installer is only for the FPGA programming interface on supported DMA
writers:

* FTDI 0403:6010, Interface 0 / Interface A (FT2232H / Digilent)
* FTDI 0403:6011, Interface 0 / Interface A (Quad RS232-HS)
* FTDI 0403:6014, Interface A (FT232H / Digilent)

Keep the writer connected. In Zadig select the exact Interface 0 device, choose
WinUSB, then click Install Driver or Replace Driver. Do not replace the driver
for another interface or an unrelated FTDI device.

Zadig 2.9 is signed by Akeo Consulting. Its complete corresponding source and
GPL/LGPL license texts are included in this folder.
