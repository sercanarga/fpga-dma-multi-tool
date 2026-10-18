#include "ui/page.hpp"
#include <algorithm>
#include <array>
namespace fpga::ui {
namespace {
class DriversPage final : public Page {
    struct DriverRow {
        Driver kind;
        const char* key;
        const wchar_t* name;
        const wchar_t* purpose;
        HWND state{}, detail{}, install{}, remove{};
        StatusTone tone = StatusTone::neutral;
    };
    std::array<DriverRow, 3> rows_{
        {{Driver::ch347, "CH347", L"CH347 driver", L"FPGA detection and programming over JTAG."},
         {Driver::d3xx, "FTDI D3XX", L"FTDI D3XX driver",
          L"DMA Speed Test with FT600 / FT601 adapters."},
         {Driver::writer, "FTDI writer", L"RS232 / FTDI writer driver",
          L"FPGA detection and programming through FTDI / Digilent Interface A."}}};
    std::optional<COLORREF> textColor(HWND control) const override {
        for (const auto& row : rows_)
            if (row.state == control)
                return statusColor(row.tone, GetSysColor(COLOR_BTNFACE));
        return std::nullopt;
    }
    void build() override {
        SetWindowLongPtrW(window_, GWL_STYLE,
                          GetWindowLongPtrW(window_, GWL_STYLE) & ~WS_CLIPCHILDREN);
        heading(L"Drivers", L"Check and manage the Windows drivers used by this application.",
                L"&Check again", 100);
        for (std::size_t i = 0; i < rows_.size(); ++i) {
            auto& row = rows_[i];
            const int y = 82 + static_cast<int>(i) * 106;
            const auto group = add(L"BUTTON", row.name, BS_GROUPBOX, 300 + static_cast<int>(i), 12,
                                   y, 12, 96, true);
            row.state =
                add(L"STATIC", L"Not checked", SS_LEFT | SS_CENTERIMAGE, 0, 24, y + 20, 200, 26);
            add(L"STATIC", row.purpose, SS_LEFT | SS_ENDELLIPSIS, 0, 24, y + 50, 24, 20, true);
            row.detail = add(L"STATIC", L"", SS_LEFT | SS_ENDELLIPSIS, 0, 24, y + 72, 24, 20, true);
            row.install = buttonRight(L"&Install", 200 + static_cast<int>(i) * 2, y + 20, 120, 152);
            row.remove = buttonRight(L"&Remove", 201 + static_cast<int>(i) * 2, y + 20, 120, 24);
            EnableWindow(row.remove, FALSE);
            SetWindowPos(group, HWND_BOTTOM, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
        }
    }
    void command(int id, int notification) override {
        if (notification != BN_CLICKED)
            return;
        if (id == 100) {
            refresh();
            return;
        }
        if (id < 200 || id > 205)
            return;
        const auto& row = rows_[static_cast<std::size_t>((id - 200) / 2)];
        const bool remove = id % 2 != 0;
        if (remove && !IsWindowEnabled(row.remove))
            return;
        const std::wstring message =
            (remove ? L"Remove " : L"Install or configure ") + std::wstring(row.name) +
            (remove ? L"? Devices using this package may stop working until it is reinstalled."
                    : L"? Windows will request administrator access.");
        if (confirm(window_, message))
            host_.submit({DriverRequest{row.kind, remove, host_.connection().serial},
                          std::chrono::minutes(10)});
    }

  public:
    using Page::Page;
    void refresh() override {
        host_.submit({InventoryRequest{Inventory::drivers}, std::chrono::seconds(90)});
    }
    void complete(const Result& result) override {
        if (std::holds_alternative<Notice>(result)) {
            refresh();
            return;
        }
        const auto* data = std::get_if<TableResult>(&result);
        if (!data)
            return;
        for (auto& row : rows_) {
            const auto found =
                std::find_if(data->rows.begin(), data->rows.end(), [&](const auto& value) {
                    return value.size() >= 3 && value[0] == row.key;
                });
            const bool known = found != data->rows.end();
            const bool installed = known && (*found)[1] == "Yes";
            row.tone = !known      ? StatusTone::neutral
                       : installed ? StatusTone::positive
                                   : StatusTone::negative;
            text(row.state, !known      ? L"Status unavailable"
                            : installed ? L"Installed"
                                        : L"Not installed");
            text(row.detail, known ? win::wide((*found)[2]) : L"Choose Check again to retry.");
            EnableWindow(row.remove, installed);
            text(row.install, installed ? L"&Reinstall" : L"&Install");
        }
    }
};
}
std::unique_ptr<Page> driversPage(PageHost& host) {
    return std::make_unique<DriversPage>(host);
}
}
