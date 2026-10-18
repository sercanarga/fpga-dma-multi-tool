#pragma once
#include "application/model.hpp"
#include "ui/table.hpp"
#include <memory>
namespace fpga::ui {
class PageHost {
  public:
    virtual ~PageHost() = default;
    virtual Connection connection() const = 0;
    virtual void submit(Request request) = 0;
    virtual void target(const Device& device) = 0;
    virtual void details(const TableResult& result) = 0;
    virtual void cancelOperation() = 0;
    virtual void status(const std::string& message) noexcept = 0;
    virtual std::string identifier(std::string value) const {
        return value;
    }
};
class Page {
    struct Placement {
        HWND window;
        int x, y, width, height;
        bool stretchWidth, stretchHeight;
        bool rightAligned = false;
        bool bottomAligned = false;
    };
    std::vector<Placement> placements_;
    HWND heading_{}, description_{}, activity_{}, activityTitle_{}, cancel_{};
    std::vector<HWND> disabled_;
    std::wstring descriptionText_;
    bool busy_ = false;
    std::optional<int> activityMinimum_;
    static constexpr int cancelId = 912;
    Font headingFont_;
    UINT headingDpi_{};
    static LRESULT CALLBACK procedure(HWND window, UINT message, WPARAM wp, LPARAM lp) noexcept;

  protected:
    HWND window_ = nullptr;
    PageHost& host_;
    Table table_;
    HWND add(const wchar_t* type, const std::wstring& text, DWORD style, int id, int x, int y,
             int width, int height, bool stretchWidth = false, bool stretchHeight = false,
             DWORD extended = 0);
    void label(const wchar_t* text, int x, int y, int width);
    void heading(const wchar_t* title, const wchar_t* description, const wchar_t* action = nullptr,
                 int actionId = 0, int actionWidth = 120, int extraWidth = 0);
    HWND buttonRight(const wchar_t* text, int id, int y, int width = 120, int inset = 12);
    HWND button(const wchar_t* text, int id, int x, int y, int width = 120);
    HWND edit(const wchar_t* text, int id, int x, int y, int width, bool stretch = false);
    HWND combo(const std::vector<std::wstring>& values, int id, int x, int y, int width,
               bool stretch = false);
    HWND buttonBottom(const wchar_t* text, int id, int x, int width);
    void table(const Row& columns, int y, int bottom = 12);
    void fixedActivity(int minimum) {
        activityMinimum_ = minimum;
    }
    bool busy() const {
        return busy_;
    }
    virtual void build() = 0;
    virtual std::optional<COLORREF> textColor(HWND) const {
        return std::nullopt;
    }
    virtual void command(int id, int notification) = 0;
    virtual void notify(const NMHDR& notification);
    virtual void openSelected();

  public:
    explicit Page(PageHost& host) : host_(host) {}
    virtual ~Page();
    Page(const Page&) = delete;
    Page& operator=(const Page&) = delete;
    void create(HWND parent);
    HWND window() const {
        return window_;
    }
    virtual void layout();
    virtual void appendLog(const std::string& message);
    void showError(const std::string& message) noexcept;
    virtual void setBusy(bool busy, bool cancellable, bool running = true);
    virtual void complete(const Result& result);
    virtual void refresh() {}
    virtual void activate() {
        refresh();
    }
    virtual void copy() {
        table_.copy(window_);
    }
    virtual void file(const std::filesystem::path&) {}
    virtual void target(const Device&) {}
    virtual void connectionChanged() {}
};
std::unique_ptr<Page> devicesPage(PageHost& host);
std::unique_ptr<Page> firmwarePage(PageHost& host);
std::unique_ptr<Page> benchmarkPage(PageHost& host);
std::unique_ptr<Page> historyPage(PageHost& host);
std::unique_ptr<Page> systemPage(PageHost& host);
std::unique_ptr<Page> driversPage(PageHost& host);
void showDetails(HWND owner, const TableResult& result);
}
