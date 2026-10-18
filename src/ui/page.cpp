#include "ui/page.hpp"
#include <algorithm>
namespace fpga::ui {
Page::~Page() {
    if (window_) {
        SetWindowLongPtrW(window_, GWLP_USERDATA, 0);
        DestroyWindow(window_);
    }
}
LRESULT CALLBACK Page::procedure(HWND window, UINT message, WPARAM wp, LPARAM lp) noexcept {
    auto* page = reinterpret_cast<Page*>(GetWindowLongPtrW(window, GWLP_USERDATA));
    if (message == WM_NCCREATE) {
        page = static_cast<Page*>(reinterpret_cast<CREATESTRUCTW*>(lp)->lpCreateParams);
        page->window_ = window;
        SetWindowLongPtrW(window, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(page));
    }
    if (!page)
        return DefWindowProcW(window, message, wp, lp);
    try {
        switch (message) {
        case WM_NCDESTROY:
            SetWindowLongPtrW(window, GWLP_USERDATA, 0);
            page->window_ = nullptr;
            return DefWindowProcW(window, message, wp, lp);
        case WM_CTLCOLORSTATIC:
        case WM_CTLCOLOREDIT:
        case WM_CTLCOLORBTN: {
            const auto brush = controlColors(message, wp, lp);
            const auto child = reinterpret_cast<HWND>(lp);
            if (message == WM_CTLCOLORSTATIC && IsWindowEnabled(child))
                if (const auto color = page->textColor(child))
                    SetTextColor(reinterpret_cast<HDC>(wp), *color);
            return brush;
        }
        case WM_COMMAND:
            if (LOWORD(wp) == cancelId) {
                page->host_.cancelOperation();
                return 0;
            }
            page->command(LOWORD(wp), HIWORD(wp));
            return 0;
        case WM_NOTIFY:
            if (const auto* header = reinterpret_cast<NMHDR*>(lp);
                header->hwndFrom == page->table_.window() && header->code == NM_CUSTOMDRAW)
                return page->table_.draw(*reinterpret_cast<NMLVCUSTOMDRAW*>(lp));
            page->notify(*reinterpret_cast<NMHDR*>(lp));
            return 0;
        case WM_SIZE:
            page->layout();
            return 0;
        default:
            break;
        }
    } catch (const std::exception& exception) {
        page->showError(exception.what());
        page->host_.status(exception.what());
    } catch (...) {
        page->showError("Unexpected interface error");
        page->host_.status("Unexpected interface error");
    }
    return DefWindowProcW(window, message, wp, lp);
}
void Page::create(HWND parent) {
    WNDCLASSW type{};
    type.lpfnWndProc = procedure;
    type.hInstance = GetModuleHandleW(nullptr);
    type.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    type.hbrBackground = GetSysColorBrush(COLOR_BTNFACE);
    type.lpszClassName = L"FpgaDmaPage";
    if (!RegisterClassW(&type) && GetLastError() != ERROR_CLASS_ALREADY_EXISTS)
        fail("Cannot register page window");
    window_ = CreateWindowExW(WS_EX_CONTROLPARENT, type.lpszClassName, L"",
                              WS_CHILD | WS_CLIPCHILDREN | WS_CLIPSIBLINGS, 0, 0, 0, 0, parent,
                              nullptr, type.hInstance, this);
    if (!window_)
        fail("Cannot create page");
    SendMessageW(window_, WM_SETFONT, SendMessageW(parent, WM_GETFONT, 0, 0), TRUE);
    build();
    if (!activityMinimum_)
        return;
    activityTitle_ = control(window_, L"STATIC", L"Activity", SS_LEFT | SS_CENTERIMAGE, 0);
    activity_ =
        control(window_, L"EDIT", L"",
                WS_TABSTOP | ES_MULTILINE | ES_READONLY | ES_AUTOVSCROLL | WS_VSCROLL | WS_HSCROLL,
                913, WS_EX_CLIENTEDGE);
    SendMessageW(activity_, EM_SETLIMITTEXT, 0x7ffffffe, 0);
    cancel_ = control(window_, L"BUTTON", L"Cancel", BS_PUSHBUTTON | WS_TABSTOP, cancelId);
    ShowWindow(cancel_, SW_HIDE);
}
void Page::setBusy(bool busy, bool cancellable, bool running) {
    const bool changed = busy_ != busy;
    busy_ = busy;
    if (busy && running)
        text(description_, descriptionText_);
    if (changed && busy) {
        disabled_.clear();
        for (const auto& placement : placements_) {
            wchar_t type[32]{};
            GetClassNameW(placement.window, type, static_cast<int>(std::size(type)));
            if (lstrcmpiW(type, L"STATIC") == 0 || lstrcmpiW(type, WC_LISTVIEWW) == 0)
                continue;
            if (lstrcmpiW(type, L"BUTTON") == 0 &&
                (GetWindowLongPtrW(placement.window, GWL_STYLE) & BS_TYPEMASK) == BS_GROUPBOX)
                continue;
            if (IsWindowEnabled(placement.window)) {
                disabled_.push_back(placement.window);
                EnableWindow(placement.window, FALSE);
            }
        }
    } else if (changed) {
        for (auto child : disabled_)
            EnableWindow(child, TRUE);
        disabled_.clear();
    }
    ShowWindow(cancel_, busy && running ? SW_SHOW : SW_HIDE);
    EnableWindow(cancel_, cancellable);
}
void Page::appendLog(const std::string& message) {
    appendText(activity_, message);
}
void Page::showError(const std::string& message) noexcept {
    try {
        appendLog(message + "\n");
        text(description_, win::wide(message.substr(0, message.find_first_of("\r\n"))));
    } catch (...) {
        SetWindowTextW(description_, L"Unable to display error details.");
    }
}
HWND Page::add(const wchar_t* type, const std::wstring& title, DWORD style, int id, int x, int y,
               int width, int height, bool stretchWidth, bool stretchHeight, DWORD extended) {
    auto child = control(window_, type, title, style, id, extended);
    placements_.push_back({child, x, y, width, height, stretchWidth, stretchHeight});
    return child;
}
void Page::heading(const wchar_t* title, const wchar_t* description, const wchar_t* action,
                   int actionId, int actionWidth, int extraWidth) {
    text(window_, title);
    heading_ = add(L"STATIC", title, SS_LEFT | SS_CENTERIMAGE, 0, 12, 14,
                   action ? actionWidth + extraWidth + 32 : 100, 28, true);
    if (action)
        buttonRight(action, actionId, 15, actionWidth);
    descriptionText_ = description;
    description_ = add(L"STATIC", description, SS_LEFT | SS_ENDELLIPSIS, 0, 12, 48, 12, 22, true);
}
HWND Page::buttonRight(const wchar_t* title, int id, int y, int width, int inset) {
    auto child = control(window_, L"BUTTON", title, WS_TABSTOP | BS_PUSHBUTTON, id);
    placements_.push_back({child, inset, y, width, 26, false, false, true});
    return child;
}
void Page::label(const wchar_t* title, int x, int y, int width) {
    add(L"STATIC", title, SS_LEFT, 0, x, y, width, 22);
}
HWND Page::button(const wchar_t* title, int id, int x, int y, int width) {
    return add(L"BUTTON", title, WS_TABSTOP | BS_PUSHBUTTON, id, x, y, width, 26);
}
HWND Page::edit(const wchar_t* value, int id, int x, int y, int width, bool stretch) {
    auto child = add(L"EDIT", value, WS_TABSTOP | ES_AUTOHSCROLL, id, x, y, width, 24, stretch,
                     false, WS_EX_CLIENTEDGE);
    SendMessageW(child, EM_SETLIMITTEXT, 4096, 0);
    return child;
}
HWND Page::combo(const std::vector<std::wstring>& values, int id, int x, int y, int width,
                 bool stretch) {
    auto child = add(WC_COMBOBOXW, L"", WS_TABSTOP | CBS_DROPDOWNLIST | WS_VSCROLL, id, x, y, width,
                     250, stretch);
    choices(child, values);
    return child;
}
HWND Page::buttonBottom(const wchar_t* title, int id, int x, int width) {
    auto child = control(window_, L"BUTTON", title, WS_TABSTOP | BS_PUSHBUTTON, id);
    placements_.push_back({child, x, 12, width, 28, false, false, false, true});
    return child;
}
void Page::table(const Row& columns, int y, int bottom) {
    table_.create(window_, 900, columns);
    text(table_.window(), text(window_));
    placements_.push_back({table_.window(), 12, y, 12, bottom, true, true});
}
void Page::layout() {
    RECT area{};
    GetClientRect(window_, &area);
    const auto s = [&](int value) { return scaled(window_, value); };
    if (activityMinimum_) {
        const int logHeight =
            std::max(0, static_cast<int>(area.bottom) - s(*activityMinimum_ + 44));
        area.bottom -= logHeight + s(36);
        move(activityTitle_, s(12), area.bottom, s(100), s(26));
        move(cancel_, area.right - s(92), area.bottom, s(80), s(26));
        move(activity_, s(12), area.bottom + s(32), area.right - s(24),
             std::max(0, logHeight - s(8)));
    }
    if (heading_) {
        const auto dpi = GetDpiForWindow(window_);
        if (dpi != headingDpi_) {
            headingFont_.update(heading_, true);
            headingDpi_ = dpi;
        }
        SendMessageW(heading_, WM_SETFONT, reinterpret_cast<WPARAM>(headingFont_.get()), TRUE);
    }
    for (const auto& p : placements_) {
        const auto x =
            p.rightAligned ? area.right - scaled(window_, p.x + p.width) : scaled(window_, p.x);
        const auto y =
            p.bottomAligned ? area.bottom - scaled(window_, p.y + p.height) : scaled(window_, p.y);
        move(p.window, x, y,
             p.stretchWidth ? area.right - x - scaled(window_, p.width) : scaled(window_, p.width),
             p.stretchHeight ? area.bottom - y - scaled(window_, p.height)
                             : scaled(window_, p.height));
    }
}
void Page::notify(const NMHDR& notification) {
    if (notification.hwndFrom != table_.window())
        return;
    if (notification.code == NM_DBLCLK)
        openSelected();
    if (notification.code == LVN_COLUMNCLICK)
        table_.sort(reinterpret_cast<const NMLISTVIEW&>(notification).iSubItem);
    if (notification.code == LVN_KEYDOWN) {
        const auto& key = reinterpret_cast<const NMLVKEYDOWN&>(notification);
        if (key.wVKey == VK_RETURN)
            openSelected();
        if (key.wVKey == 'C' && GetKeyState(VK_CONTROL) < 0)
            copy();
    }
}
void Page::openSelected() {
    const auto selected = table_.selected();
    if (!selected)
        return;
    TableResult details{{"Property", "Value"}, {}, "Selected row"};
    const auto& values = table_.rows()[*selected];
    const auto& columns = table_.columns();
    for (std::size_t i = 0; i < values.size() && i < columns.size(); ++i)
        if (!columns[i].empty())
            details.rows.push_back({columns[i], values[i]});
    host_.details(details);
}
void Page::complete(const Result& result) {
    if (const auto* value = std::get_if<TableResult>(&result))
        table_.set(value->rows);
}
}
