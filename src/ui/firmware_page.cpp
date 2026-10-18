#include "core/text.hpp"
#include "ui/page.hpp"
#include <array>
namespace fpga::ui {
namespace {
class FirmwarePage final : public Page {
    HWND path_{}, part_{}, identity_{}, program_{};
    std::array<HWND, 3> writers_{};
    std::array<HWND, 2> modes_{};
    std::optional<Device> target_;
    Backend writer_ = Backend::automatic;
    enum { browse = 100, program, pathChanged, partChanged, writerFirst = 110, modeFirst = 120 };
    template <std::size_t N>
    void selectRadio(const std::array<HWND, N>& group, std::size_t selected) {
        for (std::size_t i = 0; i < N; ++i) {
            check(group[i], i == selected);
            const auto style = GetWindowLongPtrW(group[i], GWL_STYLE);
            SetWindowLongPtrW(group[i], GWL_STYLE,
                              i == selected ? style | WS_TABSTOP : style & ~WS_TABSTOP);
        }
    }
    void selectWriter(Backend backend) {
        writer_ = backend;
        selectRadio(writers_, static_cast<std::size_t>(backend));
    }
    void build() override {
        fixedActivity(208);
        heading(L"Flash", L"Load a bitstream into temporary SRAM or write persistent flash.");
        label(L"&File:", 12, 82, 60);
        path_ = edit(L"", pathChanged, 80, 78, 140, true);
        SendMessageW(path_, EM_SETCUEBANNER, TRUE,
                     reinterpret_cast<LPARAM>(L"Choose, paste or drop a .bit / .bin file"));
        buttonRight(L"&Choose file...", browse, 78);
        label(L"&Target:", 12, 114, 60);
        std::vector<std::wstring> names;
        for (const auto& part : catalog())
            names.push_back(win::wide(part.name));
        part_ = combo(names, partChanged, 80, 110, 140, true);
        select(part_, 4);
        label(L"Writer:", 12, 146, 60);
        writers_[0] = add(L"BUTTON", L"&Auto", BS_AUTORADIOBUTTON | WS_GROUP | WS_TABSTOP,
                          writerFirst, 80, 142, 56, 26);
        writers_[1] =
            add(L"BUTTON", L"&CH347", BS_AUTORADIOBUTTON, writerFirst + 1, 140, 142, 68, 26);
        writers_[2] = add(L"BUTTON", L"&RS232 / FTDI", BS_AUTORADIOBUTTON, writerFirst + 2, 212,
                          142, 104, 26);
        label(L"Mode:", 340, 146, 48);
        modes_[0] = add(L"BUTTON", L"&Flash (.bin)", BS_AUTORADIOBUTTON | WS_GROUP | WS_TABSTOP,
                        modeFirst, 392, 142, 96, 26);
        modes_[1] =
            add(L"BUTTON", L"&SRAM (.bit)", BS_AUTORADIOBUTTON, modeFirst + 1, 492, 142, 100, 26);
        selectRadio(modes_, 0);
        identity_ = add(L"STATIC", L"Choose a firmware file to continue.",
                        SS_LEFT | SS_CENTERIMAGE | SS_ENDELLIPSIS, 0, 12, 178, 12, 26, true);
        program_ = buttonRight(L"&Program FPGA", program, 110);
        SetWindowLongPtrW(program_, GWL_STYLE, GetWindowLongPtrW(program_, GWL_STYLE) | WS_GROUP);
        EnableWindow(program_, FALSE);
        connectionChanged();
    }
    void command(int id, int notification) override {
        if (id == pathChanged && notification == EN_CHANGE) {
            EnableWindow(program_, !busy() && GetWindowTextLengthW(path_) > 0);
            const auto extension = lower(std::filesystem::path(text(path_)).extension().string());
            if (extension == ".bit" || extension == ".bin")
                selectRadio(modes_, extension == ".bit" ? 1 : 0);
            return;
        }
        if (id == partChanged && notification == CBN_SELCHANGE) {
            target_.reset();
            text(identity_, L"Target selected manually. Chain index 0 will be used.");
            return;
        }
        if (id >= writerFirst && id < writerFirst + static_cast<int>(writers_.size()) &&
            notification == BN_CLICKED) {
            const auto backend = static_cast<Backend>(id - writerFirst);
            if (backend != writer_) {
                target_.reset();
                text(identity_, L"");
            }
            selectWriter(backend);
            return;
        }
        if (notification != BN_CLICKED)
            return;
        if (id >= modeFirst && id < modeFirst + static_cast<int>(modes_.size())) {
            selectRadio(modes_, static_cast<std::size_t>(id - modeFirst));
            return;
        }
        if (id == browse) {
            if (auto chosen = chooseFile(window_))
                file(*chosen);
            return;
        }
        if (id != program)
            return;
        Firmware firmware{std::filesystem::path(text(path_)),
                          checked(modes_[1]) ? Destination::sram : Destination::flash,
                          win::utf8(text(part_)), target_ ? target_->index : 0};
        firmware.validate();
        auto connection = host_.connection();
        connection.backend = writer_;
        connection.validate();
        if (!confirm(window_, firmware.destination == Destination::flash
                                  ? L"Write and verify persistent flash on the selected FPGA?"
                                  : L"Load the selected firmware into FPGA SRAM?"))
            return;
        host_.submit({ProgramRequest{std::move(connection), std::move(firmware),
                                     target_ ? std::optional{target_->dna} : std::nullopt},
                      std::chrono::minutes(15)});
    }

  public:
    using Page::Page;
    void setBusy(bool value, bool cancellable, bool running = true) override {
        Page::setBusy(value, cancellable, running);
        EnableWindow(program_, !value && GetWindowTextLengthW(path_) > 0);
    }
    void file(const std::filesystem::path& path) override {
        if (!firmwareExtension(path) || !std::filesystem::is_regular_file(path))
            fail("Select one local .bit or .bin firmware file");
        text(path_, std::filesystem::absolute(path).native());
        selectRadio(modes_, lower(path.extension().string()) == ".bin" ? 0 : 1);
        if (!target_)
            text(identity_, L"Ready. Programming starts only after confirmation.");
    }
    void target(const Device& device) override {
        target_ = device;
        for (std::size_t i = 0; i < catalog().size(); ++i)
            if (device.part == catalog()[i].name)
                select(part_, static_cast<int>(i));
        selectWriter(host_.connection().backend);
        text(identity_, win::wide("DNA " + host_.identifier(hex(device.dna, 16)) + "  /  Chain " +
                                  std::to_string(device.index)));
    }
    void connectionChanged() override {
        target_.reset();
        selectWriter(host_.connection().backend);
    }
    void complete(const Result& result) override {
        if (const auto* value = std::get_if<ProgramResult>(&result))
            text(identity_, win::wide(resultMessage(result) + "  /  DNA " +
                                      host_.identifier(hex(value->device.dna, 16))));
    }
};
}
std::unique_ptr<Page> firmwarePage(PageHost& host) {
    return std::make_unique<FirmwarePage>(host);
}
}
