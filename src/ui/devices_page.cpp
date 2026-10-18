#include "core/text.hpp"
#include "ui/page.hpp"
#include <array>
namespace fpga::ui {
namespace {
class DevicesPage final : public Page {
    std::vector<Device> devices_;
    HWND more_{}, summary_{};
    std::array<HWND, 3> actions_{};
    enum { scan = 100, copyDna, program, registers, sensors, flash, more };
    void build() override {
        heading(L"FPGA Devices", L"Read the JTAG chain, FPGA model, IDCODE and factory DNA.",
                L"&Scan device", scan, 120);
        summary_ = add(L"STATIC", L"Connect the adapter and power the FPGA board.",
                       SS_LEFT | SS_CENTERIMAGE | SS_ENDELLIPSIS, 0, 12, 82, 12, 28, true);
        table({"#", "FPGA", "IDCODE", "DNA ID", "FUSE_DNA", "Connection", "Board"}, 120, 52);
        actions_[0] = buttonBottom(L"&Copy DNA", copyDna, 12, 160);
        actions_[1] = buttonBottom(L"&Use for programming", program, 180, 160);
        more_ = actions_[2] = buttonBottom(L"&Inspect...", more, 348, 160);
        updateActions();
    }
    void updateActions() {
        for (const auto action : actions_)
            EnableWindow(action, !busy() && table_.selected().has_value());
    }

    void openSelected() override {}
    void showActions() {
        struct Menu {
            HMENU handle = CreatePopupMenu();
            ~Menu() {
                if (handle)
                    DestroyMenu(handle);
            }
        } menu;
        if (!menu.handle)
            fail("Cannot open device actions");
        for (const auto& [id, title] : {std::pair{registers, L"Configuration registers"},
                                        {sensors, L"Sensors"},
                                        {flash, L"Flash information"}})
            AppendMenuW(menu.handle, MF_STRING, id, title);
        RECT bounds{};
        GetWindowRect(more_, &bounds);
        const auto action =
            TrackPopupMenuEx(menu.handle, TPM_RETURNCMD | TPM_LEFTALIGN | TPM_TOPALIGN, bounds.left,
                             bounds.bottom, window_, nullptr);
        if (action)
            command(static_cast<int>(action), BN_CLICKED);
    }
    void notify(const NMHDR& notification) override {
        Page::notify(notification);
        if (notification.hwndFrom != table_.window())
            return;
        if (notification.code == LVN_ITEMCHANGED)
            updateActions();
    }
    const Device& selected() const {
        auto index = table_.selected();
        if (!index || *index >= devices_.size())
            fail("Select an FPGA device");
        return devices_[*index];
    }
    void command(int id, int notification) override {
        if (notification != BN_CLICKED)
            return;
        if (id == more) {
            showActions();
            return;
        }
        if (id == scan) {
            refresh();
            return;
        }
        if (id == copyDna) {
            clipboard(window_, win::wide(host_.identifier(hex(selected().dna, 16))));
            return;
        }
        if (id == program) {
            host_.target(selected());
            return;
        }
        if (id == registers || id == sensors || id == flash) {
            const auto& device = selected();
            if (id != registers &&
                !confirm(window_, L"This inspection changes FPGA configuration state. Continue?"))
                return;
            host_.submit({InspectRequest{host_.connection(), device.index,
                                         id == registers ? Inspection::registers
                                         : id == sensors ? Inspection::sensors
                                                         : Inspection::flash,
                                         device.dna},
                          std::chrono::seconds(90)});
        }
    }

  public:
    using Page::Page;
    void setBusy(bool value, bool cancellable, bool running = true) override {
        Page::setBusy(value, cancellable, running);
        updateActions();
    }
    void refresh() override {
        host_.submit({ScanRequest{host_.connection(), -1}, std::chrono::seconds(90)});
    }
    void complete(const Result& result) override {
        if (const auto* scanResult = std::get_if<ScanResult>(&result)) {
            devices_ = scanResult->devices;
            Rows rows;
            for (const auto& d : devices_)
                rows.push_back({std::to_string(d.index), d.part, "0x" + hex(d.id, 8),
                                host_.identifier(hex(d.dna, 16)),
                                host_.identifier("0x" + hex(d.fuse, 16)), d.target,
                                d.boards.empty() ? "Unknown board" : join(d.boards, ", ")});
            table_.set(std::move(rows));
            text(summary_, win::wide(std::to_string(devices_.size()) + " device(s)"));
            if (!devices_.empty())
                ListView_SetItemState(table_.window(), 0, LVIS_SELECTED | LVIS_FOCUSED,
                                      LVIS_SELECTED | LVIS_FOCUSED);
            updateActions();
        } else if (const auto* data = std::get_if<TableResult>(&result))
            host_.details(*data);
    }
    void connectionChanged() override {
        devices_.clear();
        table_.set({});
        text(summary_, L"Ready to scan");
        updateActions();
    }
};
}
std::unique_ptr<Page> devicesPage(PageHost& host) {
    return std::make_unique<DevicesPage>(host);
}
}
