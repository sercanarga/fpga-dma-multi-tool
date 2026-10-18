#pragma once
#include "application/work_queue.hpp"
#include "ui/page_catalog.hpp"
#include <array>
namespace fpga::ui {
struct WindowOptions {
    bool refreshOnActivation = true;
    bool persistSettings = true;
    bool operationsEnabled = true;
    bool hideIdentifiers = false;
};
class Window final : public PageHost {
    HWND window_{}, tabs_{}, status_{}, credit_{}, progress_{};
    Connection connection_;
    WindowOptions options_;
    HWND operationFocus_{};
    HFONT fontHandle_ = nullptr;
    Font font_;
    std::array<std::unique_ptr<Page>, pageCount> pages_;
    std::unique_ptr<WorkQueue> work_;
    struct ActiveOperation {
        Request request;
        std::size_t page;
    };
    std::optional<ActiveOperation> active_;
    std::array<std::optional<Result>, pageCount> results_;
    std::filesystem::path settingsPath_;
    enum Command {
        openId = 1000,
        exportId,
        refreshId,
        cancelId,
        creditId,
        tabFirst = 1100,
        tabId = 1204
    };
    static LRESULT CALLBACK procedure(HWND, UINT, WPARAM, LPARAM) noexcept;
    LRESULT handle(UINT, WPARAM, LPARAM);
    void createControls();
    void layout();
    void selectTab(int index);
    void command(int id, int notification);
    void drain();
    void setBusy(bool busy);
    bool navigationAllowed() const;
    void reportError(const std::string& message) noexcept;
    void cancel();
    void loadSettings();
    void saveSettings();
    void exportResult();

  public:
    explicit Window(WindowOptions options = {}) : options_(options) {}
    ~Window();
    Window(const Window&) = delete;
    Window& operator=(const Window&) = delete;
    HWND create(int show = SW_SHOWNORMAL);
    void showPage(PageId page);
    void present(PageId page, Result result);
    void presentLog(PageId page, const std::string& message);
    std::string identifier(std::string value) const override;

    HWND window() const {
        return window_;
    }
    bool translate(MSG& message);
    void openFile(const std::filesystem::path& path);
    Connection connection() const override;
    void cancelOperation() override {
        cancel();
    }
    void submit(Request request) override;
    void status(const std::string& message) noexcept override;
    void target(const Device& device) override;
    void details(const TableResult& result) override;
};
int runGui(int show);
}
