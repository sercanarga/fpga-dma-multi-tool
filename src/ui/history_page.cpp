#include "application/history.hpp"
#include "ui/page.hpp"
#include <utility>
namespace fpga::ui {
namespace {
class HistoryPage final : public Page {
    HWND filter_{}, disconnected_{}, remove_{};
    bool updating_ = false;
    HistorySelection selection_;
    class Updating {
        bool& value_;
        bool previous_;

      public:
        explicit Updating(bool& value) : value_(value), previous_(std::exchange(value, true)) {}
        ~Updating() {
            value_ = previous_;
        }
        Updating(const Updating&) = delete;
        Updating& operator=(const Updating&) = delete;
    };
    enum { refreshId = 100, removeId = 101, filterId = 103, disconnectedId = 104 };
    void build() override {
        heading(L"Device History", L"Review connected and previously connected devices.",
                L"&Refresh", refreshId);
        label(L"&Filter:", 12, 86, 50);
        filter_ = edit(L"", filterId, 65, 82, 200);
        disconnected_ = add(L"BUTTON", L"&Disconnected only", BS_AUTOCHECKBOX | WS_TABSTOP,
                            disconnectedId, 280, 82, 170, 24);
        remove_ = buttonRight(L"&Remove selected", removeId, 82, 140, 12);
        EnableWindow(remove_, FALSE);
        check(disconnected_, true);
        table({"", "Name", "Class", "Connected", "Enumerator", "Instance"}, 118);
        ListView_SetExtendedListViewStyleEx(table_.window(), LVS_EX_CHECKBOXES, LVS_EX_CHECKBOXES);
        const auto header = ListView_GetHeader(table_.window());
        SetWindowLongPtrW(header, GWL_STYLE, GetWindowLongPtrW(header, GWL_STYLE) | HDS_CHECKBOXES);
        HDITEMW item{};
        item.mask = HDI_FORMAT;
        Header_GetItem(header, 0, &item);
        item.fmt |= HDF_CHECKBOX;
        Header_SetItem(header, 0, &item);
    }
    void updateSelection() {
        const auto header = ListView_GetHeader(table_.window());
        HDITEMW item{};
        item.mask = HDI_FORMAT;
        if (Header_GetItem(header, 0, &item)) {
            const auto format =
                selection_.allChecked() ? item.fmt | HDF_CHECKED : item.fmt & ~HDF_CHECKED;
            if (item.fmt != format) {
                item.fmt = format;
                Header_SetItem(header, 0, &item);
            }
        }
        EnableWindow(remove_, !busy() && !selection_.checked().empty());
    }
    void notify(const NMHDR& notification) override {
        const auto header = ListView_GetHeader(table_.window());
        if (notification.code == HDN_ITEMSTATEICONCLICK &&
            (notification.hwndFrom == header || notification.hwndFrom == table_.window()) &&
            reinterpret_cast<const NMHEADERW&>(notification).iItem == 0) {
            if (!busy() && !updating_) {
                Updating guard(updating_);
                selection_.toggleAll();
                table_.checkRows(selection_.checked());
            }
            updateSelection();
            return;
        }
        if (updating_ || notification.hwndFrom != table_.window() ||
            (notification.code != LVN_ITEMCHANGED && notification.code != LVN_COLUMNCLICK)) {
            Page::notify(notification);
            return;
        }
        Updating guard(updating_);
        Page::notify(notification);
        if (!busy())
            selection_.check(table_.checkedRows());
        table_.checkRows(selection_.checked());
        updateSelection();
    }
    void render() {
        Updating guard(updating_);
        selection_.filter(win::utf8(text(filter_)), checked(disconnected_));
        Rows rows;
        for (const auto& device : selection_.visible())
            rows.push_back({"", device.name, device.deviceClass, device.present ? "Yes" : "No",
                            device.enumerator, host_.identifier(device.instance)});
        table_.set(std::move(rows));
        updateSelection();
    }
    void command(int id, int notification) override {
        if ((id == filterId && notification == EN_CHANGE) ||
            (id == disconnectedId && notification == BN_CLICKED)) {
            render();
            return;
        }
        if (notification != BN_CLICKED)
            return;
        if (id == refreshId)
            refresh();
        else if (id == removeId && !busy()) {
            auto removal = selection_.removal();
            const auto count = removal.instances.size();
            Request request{std::move(removal), std::chrono::minutes(10)};
            request.validate();
            if (confirm(window_, win::wide("Remove " + std::to_string(count) +
                                           " checked disconnected device instance(s)?")))
                host_.submit(std::move(request));
        }
    }

  public:
    using Page::Page;
    void setBusy(bool value, bool cancellable, bool running = true) override {
        Page::setBusy(value, cancellable, running);
        updateSelection();
    }
    void refresh() override {
        host_.submit({InventoryRequest{Inventory::history}, std::chrono::seconds(90)});
    }
    void complete(const Result& result) override {
        if (const auto* data = std::get_if<HistoryResult>(&result)) {
            selection_.reset(data->devices);
            render();
        } else if (std::holds_alternative<Notice>(result))
            refresh();
    }
};
}
std::unique_ptr<Page> historyPage(PageHost& host) {
    return std::make_unique<HistoryPage>(host);
}
}
