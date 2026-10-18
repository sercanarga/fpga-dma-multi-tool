#include "ui/window.hpp"
#include <dwmapi.h>
#include <iostream>
#include <wincodec.h>
#include <wrl/client.h>
namespace {
using Microsoft::WRL::ComPtr;
void check(HRESULT result) {
    if (FAILED(result))
        fpga::fail("Screenshot encoding failed: " + std::to_string(result));
}
class Capture {
    HWND window_;
    HDC source_, target_;
    HBITMAP bitmap_;
    HGDIOBJ previous_;

  public:
    explicit Capture(HWND window)
        : window_(window), source_(GetWindowDC(window)), target_(CreateCompatibleDC(source_)),
          bitmap_(nullptr), previous_(nullptr) {
        RECT area{};
        GetWindowRect(window, &area);
        if (source_ && target_) {
            bitmap_ =
                CreateCompatibleBitmap(source_, area.right - area.left, area.bottom - area.top);
            if (bitmap_)
                previous_ = SelectObject(target_, bitmap_);
        }
    }
    ~Capture() {
        if (previous_)
            SelectObject(target_, previous_);
        if (bitmap_)
            DeleteObject(bitmap_);
        if (target_)
            DeleteDC(target_);
        if (source_)
            ReleaseDC(window_, source_);
    }
    Capture(const Capture&) = delete;
    Capture& operator=(const Capture&) = delete;
    void save(const std::filesystem::path& path) const {
        RECT outer{}, visible{};
        if (!GetWindowRect(window_, &outer))
            fpga::fail("Cannot read documentation window bounds");
        check(
            DwmGetWindowAttribute(window_, DWMWA_EXTENDED_FRAME_BOUNDS, &visible, sizeof(visible)));
        if (!bitmap_ || !previous_ || previous_ == HGDI_ERROR ||
            !BitBlt(target_, 0, 0, outer.right - outer.left, outer.bottom - outer.top, source_, 0,
                    0, SRCCOPY | CAPTUREBLT))
            fpga::fail("Cannot capture documentation window");
        ComPtr<IWICImagingFactory> factory;
        check(CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER,
                               IID_PPV_ARGS(&factory)));
        ComPtr<IWICBitmap> image;
        check(factory->CreateBitmapFromHBITMAP(bitmap_, nullptr, WICBitmapIgnoreAlpha, &image));
        WICRect bounds{static_cast<int>(visible.left - outer.left),
                       static_cast<int>(visible.top - outer.top),
                       static_cast<int>(visible.right - visible.left),
                       static_cast<int>(visible.bottom - visible.top)};
        ComPtr<IWICBitmapClipper> frameImage;
        check(factory->CreateBitmapClipper(&frameImage));
        check(frameImage->Initialize(image.Get(), &bounds));
        ComPtr<IWICStream> stream;
        check(factory->CreateStream(&stream));
        check(stream->InitializeFromFilename(path.c_str(), GENERIC_WRITE));
        ComPtr<IWICBitmapEncoder> encoder;
        check(factory->CreateEncoder(GUID_ContainerFormatPng, nullptr, &encoder));
        check(encoder->Initialize(stream.Get(), WICBitmapEncoderNoCache));
        ComPtr<IWICBitmapFrameEncode> frame;
        check(encoder->CreateNewFrame(&frame, nullptr));
        check(frame->Initialize(nullptr));
        check(frame->WriteSource(frameImage.Get(), nullptr));
        check(frame->Commit());
        check(encoder->Commit());
    }
};
void paint(HWND window) {
    for (int i = 0; i < 12; ++i) {
        MSG message{};
        while (PeekMessageW(&message, nullptr, 0, 0, PM_REMOVE)) {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
        RedrawWindow(window, nullptr, nullptr,
                     RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_UPDATENOW);
        DwmFlush();
        Sleep(20);
    }
}
void populate(fpga::ui::Window& window) {
    using namespace fpga;
    using ui::PageId;
    const Device device{0,          "xc7a75t",         "CH347",           "CH347",
                        0x03632093, 0x123456789abcdef, 0x123456789abcdef, {"Enigma X1"}};
    window.present(PageId::devices, ScanResult{{device}});
    window.target(device);
    window.presentLog(PageId::flash, "Target selected: xc7a75t\nDNA: ****************\nChoose a "
                                     ".bin file for flash or a .bit file for SRAM.\n");
    window.presentLog(PageId::speed, "Example output\n\nRandom reads | 8 B | 16 MiB range\nEach "
                                     "sample reads a fresh 4 KiB DMA page.\n\n"
                                     "Read (8 B): 6324 reads/s | 0.05 MiB/s | 158.13 us\n"
                                     "Read (8 B): 6465 reads/s | 0.05 MiB/s | 154.68 us\n"
                                     "Read (8 B): 6381 reads/s | 0.05 MiB/s | 156.72 us\n");
    window.present(
        PageId::history,
        HistoryResult{
            {{"USB\\VID_1A86&PID_55DD\\EXAMPLE1", "USB-Enhanced-SERIAL CH347", "Ports", "USB",
              false},
             {"USB\\VID_0403&PID_601F\\EXAMPLE2", "FTDI FT601 USB 3.0 Bridge", "USB", "USB", false},
             {"USB\\VID_0403&PID_6010\\EXAMPLE3", "USB Serial Converter A", "USB", "USB", false}}});
    window.present(PageId::system, TableResult{{"Property", "Value"},
                                               {{"System model", "Desktop PC"},
                                                {"Firmware", "UEFI"},
                                                {"TPM", "Ready"},
                                                {"Windows edition", "Windows 11 Pro"},
                                                {"Windows architecture", "64-bit"},
                                                {"Secure Boot", "Enabled"},
                                                {"Virtualization", "Enabled"},
                                                {"Memory integrity", "Disabled"},
                                                {"DMA protection", "Off"},
                                                {"PCIe link", "PCI Express x1"}},
                                               "System information loaded"});
    window.present(
        PageId::drivers,
        TableResult{{"Driver", "Installed", "Detail"},
                    {{"CH347", "Yes", "WCH CH347 device driver"},
                     {"FTDI D3XX", "Yes", "FTDI USB 3.0 D3XX driver"},
                     {"FTDI writer", "No", "Select Install to configure Interface A with Zadig."}},
                    "Driver status checked"});
}
}
int wmain(int argc, wchar_t** argv) {
    try {
        if (argc != 2)
            fpga::fail("Usage: documentation_screenshots <output-directory>");
        const std::filesystem::path output(argv[1]);
        std::filesystem::create_directories(output);
        fpga::win::ComApartment apartment(COINIT_APARTMENTTHREADED);
        fpga::ui::Window window({.refreshOnActivation = false,
                                 .persistSettings = false,
                                 .operationsEnabled = false,
                                 .hideIdentifiers = true});
        const auto handle = window.create();
        populate(window);
        const std::array names{"devices",        "flash",       "speed-test",
                               "device-history", "system-info", "drivers"};
        static_assert(names.size() == fpga::ui::pageCount);
        for (std::size_t i = 0; i < names.size(); ++i) {
            window.showPage(static_cast<fpga::ui::PageId>(i));
            window.status("Documentation preview");
            SetForegroundWindow(handle);
            SetFocus(GetDlgItem(handle, 1204));
            paint(handle);
            if (GetForegroundWindow() != handle)
                fpga::fail("Keep the documentation window in the foreground during capture");
            Capture(handle).save(output / (std::string(names[i]) + ".png"));
        }
        std::cout << "Saved six screenshots with synthetic data and masked identifiers\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
