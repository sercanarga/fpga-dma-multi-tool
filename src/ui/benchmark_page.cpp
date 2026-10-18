#include "core/text.hpp"
#include "ui/page.hpp"
#include <array>
namespace fpga::ui {
namespace {
class BenchmarkPage final : public Page {
    HWND mode_{}, duration_{}, sizes_{}, address_{}, length_{}, dedicated_{}, pattern_{};
    HWND advanced_{}, output_{}, start_{}, stop_{};
    std::vector<HWND> options_;
    bool expanded_ = false;

    void optionLabel(const wchar_t* title, int x, int y, int width) {
        options_.push_back(add(L"STATIC", title, SS_LEFT, 0, x, y, width, 22));
    }
    void expand(bool expanded) {
        expanded_ = expanded;
        for (auto option : options_)
            ShowWindow(option, expanded ? SW_SHOW : SW_HIDE);
        layout();
        RedrawWindow(window_, nullptr, nullptr,
                     RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_FRAME);
    }
    void build() override {
        heading(L"Speed Test", L"Random 8-byte reads. Results show reads per second.",
                L"&Start test", 100, 120, 128);
        start_ = GetDlgItem(window_, 100);
        stop_ = control(window_, L"BUTTON", L"S&top", BS_PUSHBUTTON | WS_TABSTOP, 108);
        ShowWindow(stop_, SW_HIDE);
        advanced_ = buttonRight(L"&Advanced", 107, 15, 120, 140);
        optionLabel(L"&Operation:", 12, 82, 80);
        mode_ = combo({L"Read", L"Write", L"Read and write"}, 101, 100, 78, 240);
        optionLabel(L"&Duration:", 368, 82, 100);
        duration_ =
            combo({L"Until stopped", L"3 seconds", L"10 seconds", L"30 seconds", L"60 seconds"},
                  102, 480, 78, 240);
        optionLabel(L"&Address:", 12, 118, 80);
        address_ = edit(L"", 103, 100, 114, 240);
        SendMessageW(address_, EM_SETCUEBANNER, FALSE,
                     reinterpret_cast<LPARAM>(L"Automatic (optional override)"));
        optionLabel(L"Read &pattern:", 368, 118, 100);
        pattern_ = combo({L"Random addresses", L"Same address"}, 110, 480, 114, 240);
        optionLabel(L"Transfer si&ze:", 12, 154, 80);
        sizes_ = combo({L"8 bytes (Neko default)", L"4 KiB", L"64 KiB", L"1 MiB", L"16 MiB",
                        L"4 KiB, 64 KiB, 1 MiB"},
                       105, 100, 150, 240);
        optionLabel(L"&Buffer (MiB):", 368, 154, 100);
        length_ = edit(L"16", 104, 480, 150, 240);
        dedicated_ = add(L"BUTTON", L"Write tests use a dedicated, idle buffer.",
                         BS_AUTOCHECKBOX | WS_TABSTOP, 106, 12, 190, 12, 26, true);
        options_.insert(options_.end(),
                        {mode_, duration_, address_, length_, sizes_, dedicated_, pattern_});
        output_ = control(window_, L"EDIT", L"",
                          WS_TABSTOP | ES_MULTILINE | ES_READONLY | ES_AUTOVSCROLL |
                              ES_AUTOHSCROLL | WS_VSCROLL | WS_HSCROLL,
                          109, WS_EX_CLIENTEDGE);
        SendMessageW(output_, EM_SETLIMITTEXT, 0x7ffffffe, 0);
        expand(false);
    }
    void command(int id, int notification) override {
        if ((id == 101 || id == 105) && notification == CBN_SELCHANGE) {
            if ((selection(mode_) != 0 || selection(sizes_) == 5) && selection(duration_) == 0)
                select(duration_, 1);
            return;
        }
        if (notification != BN_CLICKED)
            return;
        if (id == 107) {
            expand(!expanded_);
            return;
        }
        if (id == 108) {
            host_.cancelOperation();
            return;
        }
        if (id != 100)
            return;
        const auto address = trim(win::utf8(text(address_)));
        if (selection(mode_) != 0 && (address.empty() || !checked(dedicated_))) {
            expand(true);
            SetFocus(text(address_).empty() ? address_ : dedicated_);
            text(output_, L"For write tests, enter a buffer address and confirm that it is "
                          L"dedicated and idle.\r\n");
            return;
        }
        Benchmark settings;
        settings.mode = static_cast<TransferMode>(selection(mode_));
        constexpr std::array durations{0, 3, 10, 30, 60};
        settings.duration = durations.at(static_cast<std::size_t>(selection(duration_)));
        auto mib = number(win::utf8(text(length_)));
        if (mib > 256)
            fail("Buffer must not exceed 256 MiB");
        settings.length = mib * 1024 * 1024;
        settings.address = address.empty() ? 0 : number(address);
        settings.dedicated = checked(dedicated_);
        settings.randomRead = selection(pattern_) == 0;
        const std::array<std::vector<int>, 6> sizes{
            {{8}, {4096}, {65536}, {1048576}, {16777216}, {4096, 65536, 1048576}}};
        settings.sizes = sizes.at(static_cast<std::size_t>(selection(sizes_)));
        settings.validate();
        auto timeout =
            settings.duration == 0
                ? std::chrono::seconds(0)
                : std::chrono::seconds(settings.duration * static_cast<int>(settings.sizes.size()) *
                                           (settings.mode == TransferMode::both ? 2 : 1) +
                                       30);
        host_.submit({BenchmarkRequest{settings}, timeout});
        text(output_, L"Opening DMA device...\r\n");
    }

  public:
    using Page::Page;
    void layout() override {
        Page::layout();
        if (!output_)
            return;
        RECT area{};
        GetClientRect(window_, &area);
        const auto s = [&](int value) { return scaled(window_, value); };
        const int top = s(expanded_ ? 230 : 82);
        move(output_, s(12), top, area.right - s(24), area.bottom - top - s(12));
        move(stop_, area.right - s(132), s(15), s(120), s(26));
    }
    void setBusy(bool busy, bool cancellable, bool running) override {
        Page::setBusy(busy, cancellable, running);
        ShowWindow(start_, busy && running ? SW_HIDE : SW_SHOW);
        ShowWindow(stop_, busy && running ? SW_SHOW : SW_HIDE);
        EnableWindow(stop_, cancellable);
        if (busy && cancellable && IsWindowVisible(stop_))
            SetFocus(stop_);
    }
    void appendLog(const std::string& message) override {
        appendText(output_, message);
    }
    void complete(const Result& result) override {
        if (const auto* data = std::get_if<BenchmarkResult>(&result)) {
            appendLog("\nAverage results\n");
            for (const auto& m : data->measurements)
                appendLog(std::string(m.operation == TransferMode::read ? "Read" : "Write") + " (" +
                          std::to_string(m.bytes) + " B): " + decimal(m.operationsPerSecond, 0) +
                          (m.operation == TransferMode::read ? " reads/s | " : " writes/s | ") +
                          decimal(m.mibPerSecond, 2) + " MiB/s | " + decimal(m.latencyUs, 2) +
                          " us\n");
        }
    }
};
}
std::unique_ptr<Page> benchmarkPage(PageHost& host) {
    return std::make_unique<BenchmarkPage>(host);
}
}
