#include "ui/page.hpp"
namespace fpga::ui {
namespace {
class SystemPage final : public Page {
    void build() override {
        heading(L"System Info", L"Windows, security and PCIe information.", L"&Refresh", 100);
        table({"Property", "Value"}, 82);
        table_.colorStatusColumn(1);
    }
    void command(int id, int notification) override {
        if (notification == BN_CLICKED) {
            if (id == 100)
                refresh();
            else if (id == 101)
                copy();
        }
    }

  public:
    using Page::Page;
    void complete(const Result& result) override {
        const auto* data = std::get_if<TableResult>(&result);
        if (!data)
            return;
        std::vector<std::pair<std::wstring, Rows>> groups{{L"System and firmware", {}},
                                                          {L"Windows", {}},
                                                          {L"Security and protection", {}},
                                                          {L"PCIe", {}}};
        for (const auto& row : data->rows) {
            if (row.empty())
                continue;
            const auto& name = row.front();
            std::size_t group = 0;
            if (name.starts_with("Windows"))
                group = 1;
            else if (name.starts_with("PCIe"))
                group = 3;
            else if (name.starts_with("Virtualization") || name.starts_with("Hypervisor") ||
                     name.starts_with("Device Guard") || name.starts_with("DMA protection") ||
                     name.starts_with("VBS") || name.starts_with("Credential Guard") ||
                     name.starts_with("Memory integrity") || name.starts_with("Secure"))
                group = 2;
            groups[group].second.push_back(row);
        }
        table_.setGroups(std::move(groups));
        layout();
    }
    void layout() override {
        Page::layout();
        if (!table_.window())
            return;
        table_.resizeColumns();
    }
    void refresh() override {
        host_.submit({InventoryRequest{Inventory::system}, std::chrono::minutes(2)});
    }
};
}
std::unique_ptr<Page> systemPage(PageHost& host) {
    return std::make_unique<SystemPage>(host);
}
}
