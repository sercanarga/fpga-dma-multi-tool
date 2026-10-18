#include "ui/window.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <shellapi.h>
#include <shlobj.h>
namespace fpga::ui {
Window::~Window() {
    work_.reset();
    if (IsWindow(window_))
        DestroyWindow(window_);
}
LRESULT CALLBACK Window::procedure(HWND window, UINT message, WPARAM wp, LPARAM lp) noexcept {
    auto* self = reinterpret_cast<Window*>(GetWindowLongPtrW(window, GWLP_USERDATA));
    if (message == WM_NCCREATE) {
        self = static_cast<Window*>(reinterpret_cast<CREATESTRUCTW*>(lp)->lpCreateParams);
        self->window_ = window;
        SetWindowLongPtrW(window, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(self));
    }
    if (!self)
        return DefWindowProcW(window, message, wp, lp);
    try {
        return self->handle(message, wp, lp);
    } catch (const std::exception& exception) {
        self->reportError(exception.what());
        return message == WM_CREATE ? -1 : 0;
    } catch (...) {
        self->reportError("Unexpected application error");
        return message == WM_CREATE ? -1 : 0;
    }
}
HWND Window::create(int show) {
    INITCOMMONCONTROLSEX controls{sizeof(controls), ICC_TAB_CLASSES | ICC_LISTVIEW_CLASSES |
                                                        ICC_BAR_CLASSES | ICC_STANDARD_CLASSES |
                                                        ICC_LINK_CLASS | ICC_PROGRESS_CLASS};
    if (!InitCommonControlsEx(&controls))
        fail("Cannot initialize Windows controls");
    WNDCLASSW type{};
    type.lpfnWndProc = procedure;
    type.hInstance = GetModuleHandleW(nullptr);
    type.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    type.hIcon = LoadIconW(type.hInstance, MAKEINTRESOURCEW(1));
    type.hbrBackground = GetSysColorBrush(COLOR_BTNFACE);
    type.lpszClassName = L"FpgaDmaMultiTool";
    if (!RegisterClassW(&type) && GetLastError() != ERROR_CLASS_ALREADY_EXISTS)
        fail("Cannot register application window");
    const auto dpi = GetDpiForSystem();
    const auto scale = [&](int value) { return MulDiv(value, static_cast<int>(dpi), 96); };
    RECT work{};
    if (!SystemParametersInfoW(SPI_GETWORKAREA, 0, &work, 0))
        fail("Cannot read desktop work area");
    const int width = std::min(scale(960), static_cast<int>(work.right - work.left) - scale(16));
    const int height = std::min(scale(680), static_cast<int>(work.bottom - work.top) - scale(16));
    auto window =
        CreateWindowExW(WS_EX_CONTROLPARENT, type.lpszClassName, L"FPGA DMA Multi Tool",
                        WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN, CW_USEDEFAULT, CW_USEDEFAULT, width,
                        height, nullptr, nullptr, type.hInstance, this);
    if (!window)
        fail("Cannot create main window");
    applicationIcons(window);
    ShowWindow(window, show);
    UpdateWindow(window);
    return window;
}
void Window::showPage(PageId page) {
    const auto index = static_cast<std::size_t>(page);
    if (index >= pageCount)
        fail("Invalid page");
    if (!navigationAllowed())
        fail("Wait for the current operation to finish");
    selectTab(static_cast<int>(index));
}
std::string Window::identifier(std::string value) const {
    return options_.hideIdentifiers ? std::string(value.size(), '*') : value;
}
void Window::selectTab(int index) {
    if (index < 0 || index >= static_cast<int>(pages_.size()))
        return;
    TabCtrl_SetCurSel(tabs_, index);
    layout();
    if (const auto focus = GetFocus(); focus && IsChild(window_, focus) && !IsWindowVisible(focus))
        SetFocus(tabs_);
    if (!active_ && options_.refreshOnActivation)
        pages_[static_cast<std::size_t>(index)]->activate();
}
bool Window::navigationAllowed() const {
    return !active_ || active_->request.inventory();
}
void Window::status(const std::string& message) noexcept {
    try {
        const auto value = win::wide(message.substr(0, message.find_first_of("\r\n")));
        SendMessageW(status_, SB_SETTEXTW, 0, reinterpret_cast<LPARAM>(value.c_str()));
    } catch (...) {
        SendMessageW(status_, SB_SETTEXTW, 0,
                     reinterpret_cast<LPARAM>(L"Unable to display status details."));
    }
}
void Window::reportError(const std::string& message) noexcept {
    const auto index = TabCtrl_GetCurSel(tabs_);
    if (index >= 0 && static_cast<std::size_t>(index) < pages_.size())
        pages_[static_cast<std::size_t>(index)]->showError(message);
    status(message);
}
Connection Window::connection() const {
    return connection_;
}
void Window::openFile(const std::filesystem::path& path) {
    if (active_)
        fail("Wait for the current operation to finish");
    pages_[1]->file(path);
    selectTab(1);
}
void Window::target(const Device& device) {
    pages_[1]->target(device);
    selectTab(1);
}
void Window::details(const TableResult& result) {
    showDetails(window_, result);
}
void Window::command(int id, int) {
    if (id >= tabFirst && id < tabFirst + static_cast<int>(pageCount)) {
        if (navigationAllowed())
            selectTab(id - tabFirst);
        return;
    }
    switch (id) {
    case openId:
        if (!active_)
            if (auto path = chooseFile(window_))
                openFile(*path);
        break;
    case exportId:
        exportResult();
        break;
    case refreshId:
        if (!active_)
            pages_[static_cast<std::size_t>(TabCtrl_GetCurSel(tabs_))]->refresh();
        break;
    case cancelId:
        cancel();
        break;
    default:
        break;
    }
}
LRESULT Window::handle(UINT message, WPARAM wp, LPARAM lp) {
    switch (message) {
    case WM_CTLCOLORSTATIC:
    case WM_CTLCOLOREDIT:
    case WM_CTLCOLORBTN:
        return controlColors(message, wp, lp);
    case WM_CREATE:
        createControls();
        return 0;
    case WM_GETFONT:
        return reinterpret_cast<LRESULT>(fontHandle_);
    case WM_SETFONT:
        fontHandle_ = reinterpret_cast<HFONT>(wp);
        return 0;
    case WM_SIZE:
        layout();
        return 0;
    case WM_COMMAND:
        command(LOWORD(wp), HIWORD(wp));
        return 0;
    case WM_NOTIFY:
        if (const auto* notification = reinterpret_cast<NMHDR*>(lp);
            notification->hwndFrom == credit_ &&
            (notification->code == NM_CLICK || notification->code == NM_RETURN)) {
            if (reinterpret_cast<INT_PTR>(ShellExecuteW(window_, L"open",
                                                        L"https://github.com/sercanarga", nullptr,
                                                        nullptr, SW_SHOWNORMAL)) <= 32)
                fail("Cannot open GitHub profile");
            return 0;
        }
        if (const auto* notification = reinterpret_cast<NMHDR*>(lp);
            notification->hwndFrom == tabs_) {
            if (notification->code == TCN_SELCHANGING)
                return !navigationAllowed();
            if (notification->code == TCN_SELCHANGE)
                selectTab(TabCtrl_GetCurSel(tabs_));
        }
        return 0;
    case WorkQueue::message:
        drain();
        return 0;
    case WM_TIMER:
        if (wp == 1)
            drain();
        return 0;
    case WM_GETMINMAXINFO: {
        auto* limits = reinterpret_cast<MINMAXINFO*>(lp);
        limits->ptMinTrackSize = {scaled(window_, 820), scaled(window_, 590)};
        return 0;
    }
    case WM_DPICHANGED: {
        const auto* rectangle = reinterpret_cast<RECT*>(lp);
        SetWindowPos(window_, nullptr, rectangle->left, rectangle->top,
                     rectangle->right - rectangle->left, rectangle->bottom - rectangle->top,
                     SWP_NOZORDER);
        font_.update(window_);
        applicationIcons(window_);
        layout();
        return 0;
    }
    case WM_SYSCOLORCHANGE:
    case WM_THEMECHANGED:
        refreshSystemColors(window_);
        return DefWindowProcW(window_, message, wp, lp);
    case WM_SETTINGCHANGE:
        font_.update(window_);
        applicationIcons(window_);
        layout();
        return 0;
    case WM_DROPFILES: {
        const auto drop = reinterpret_cast<HDROP>(wp);
        std::optional<std::filesystem::path> path;
        if (DragQueryFileW(drop, 0xffffffff, nullptr, 0) == 1) {
            const auto size = DragQueryFileW(drop, 0, nullptr, 0);
            std::wstring value(static_cast<std::size_t>(size) + 1, L'\0');
            DragQueryFileW(drop, 0, value.data(), size + 1);
            value.resize(size);
            path = value;
        }
        DragFinish(drop);
        if (path)
            openFile(*path);
        else
            fail("Drop exactly one firmware file");
        return 0;
    }
    case WM_CLOSE:
        if (active_) {
            if (active_->request.privileged()) {
                status("Finish the Windows driver/device operation before closing.");
                return 0;
            }
            if (!confirm(window_, L"Cancel the running operation and close?"))
                return 0;
            work_->cancel();
        }
        saveSettings();
        work_.reset();
        DestroyWindow(window_);
        return 0;
    case WM_NCDESTROY: {
        const auto window = window_;
        SetWindowLongPtrW(window, GWLP_USERDATA, 0);
        window_ = nullptr;
        return DefWindowProcW(window, message, wp, lp);
    }
    case WM_DESTROY:
        KillTimer(window_, 1);
        PostQuitMessage(0);
        return 0;
    default:
        return DefWindowProcW(window_, message, wp, lp);
    }
}
bool Window::translate(MSG& message) {
    if (message.message == WM_KEYDOWN) {
        const bool controlKey = GetKeyState(VK_CONTROL) < 0;
        if (controlKey && (message.wParam == 'O' || message.wParam == 'S')) {
            SendMessageW(window_, WM_COMMAND, message.wParam == 'O' ? openId : exportId, 0);
            return true;
        }
        if (message.wParam == VK_F5) {
            SendMessageW(window_, WM_COMMAND, refreshId, 0);
            return true;
        }
        if (message.wParam == VK_ESCAPE) {
            SendMessageW(window_, WM_COMMAND, cancelId, 0);
            return true;
        }
        if (controlKey && message.wParam == VK_TAB && navigationAllowed()) {
            const int direction = GetKeyState(VK_SHIFT) < 0 ? -1 : 1;
            selectTab((TabCtrl_GetCurSel(tabs_) + direction + static_cast<int>(pageCount)) %
                      static_cast<int>(pageCount));
            return true;
        }
    }
    return IsDialogMessageW(window_, &message) != FALSE;
}
int runGui(int show) {
    Window window;
    window.create(show);
    MSG message{};
    for (;;) {
        auto result = GetMessageW(&message, nullptr, 0, 0);
        if (result == 0)
            return static_cast<int>(message.wParam);
        if (result < 0)
            fail("Windows message loop failed");
        if (!window.translate(message)) {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
}
}
