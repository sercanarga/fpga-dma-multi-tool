#include "ui/page.hpp"
namespace fpga::ui {
namespace {
struct DetailWindow {
    HWND window{}, close{}, copy{}, notice{};
    Table table;
    Font font;
    const TableResult& result;
    explicit DetailWindow(const TableResult& data) : result(data) {}
    void layout() {
        RECT area{};
        GetClientRect(window, &area);
        const auto margin = scaled(window, 12), buttonWidth = scaled(window, 120),
                   buttonHeight = scaled(window, 26);
        move(table.window(), margin, margin, area.right - 2 * margin,
             area.bottom - 3 * margin - buttonHeight);
        move(copy, margin, area.bottom - margin - buttonHeight, buttonWidth, buttonHeight);
        move(close, area.right - margin - buttonWidth, area.bottom - margin - buttonHeight,
             buttonWidth, buttonHeight);
        move(notice, 2 * margin + buttonWidth, area.bottom - margin - buttonHeight,
             area.right - 4 * margin - 2 * buttonWidth, buttonHeight);
    }
    static LRESULT CALLBACK procedure(HWND window, UINT message, WPARAM wp, LPARAM lp) noexcept {
        auto* self = reinterpret_cast<DetailWindow*>(GetWindowLongPtrW(window, GWLP_USERDATA));
        if (message == WM_NCCREATE) {
            self = static_cast<DetailWindow*>(reinterpret_cast<CREATESTRUCTW*>(lp)->lpCreateParams);
            self->window = window;
            SetWindowLongPtrW(window, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(self));
        }
        if (!self)
            return DefWindowProcW(window, message, wp, lp);
        try {
            switch (message) {
            case WM_NCDESTROY:
                SetWindowLongPtrW(window, GWLP_USERDATA, 0);
                self->window = nullptr;
                return DefWindowProcW(window, message, wp, lp);
            case WM_SYSCOLORCHANGE:
            case WM_THEMECHANGED:
                refreshSystemColors(window);
                return DefWindowProcW(window, message, wp, lp);
            case WM_CREATE:
                self->table.create(window, 900, self->result.columns);
                self->table.set(self->result.rows);
                self->copy = control(window, L"BUTTON", L"&Copy", WS_TABSTOP | BS_PUSHBUTTON, 100);
                self->close =
                    control(window, L"BUTTON", L"Close", WS_TABSTOP | BS_DEFPUSHBUTTON, IDOK);
                self->notice =
                    control(window, L"STATIC", L"", SS_LEFT | SS_CENTERIMAGE | SS_ENDELLIPSIS, 0);
                self->font.update(window);
                applicationIcons(window);
                return 0;
            case WM_SIZE:
                self->layout();
                return 0;
            case WM_COMMAND:
                if (LOWORD(wp) == IDOK || LOWORD(wp) == IDCANCEL)
                    DestroyWindow(window);
                else if (LOWORD(wp) == 100)
                    self->table.copy(window);
                return 0;
            case WM_NOTIFY:
                if (reinterpret_cast<NMHDR*>(lp)->code == LVN_COLUMNCLICK)
                    self->table.sort(reinterpret_cast<NMLISTVIEW*>(lp)->iSubItem);
                return 0;
            case WM_DPICHANGED: {
                const auto* rectangle = reinterpret_cast<RECT*>(lp);
                SetWindowPos(window, nullptr, rectangle->left, rectangle->top,
                             rectangle->right - rectangle->left, rectangle->bottom - rectangle->top,
                             SWP_NOZORDER);
                self->font.update(window);
                applicationIcons(window);
                self->layout();
                return 0;
            }
            default:
                break;
            }
        } catch (const std::exception& exception) {
            try {
                text(self->notice, win::wide(exception.what()));
            } catch (...) {
                SetWindowTextW(self->notice, L"Unable to display error details.");
            }
            if (message == WM_CREATE)
                return -1;
        }
        return DefWindowProcW(window, message, wp, lp);
    }
};
}
void showDetails(HWND owner, const TableResult& result) {
    WNDCLASSW type{};
    type.lpfnWndProc = DetailWindow::procedure;
    type.hInstance = GetModuleHandleW(nullptr);
    type.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    type.hbrBackground = GetSysColorBrush(COLOR_BTNFACE);
    type.lpszClassName = L"FpgaDmaDetails";
    if (!RegisterClassW(&type) && GetLastError() != ERROR_CLASS_ALREADY_EXISTS)
        fail("Cannot register details window");
    DetailWindow dialog(result);
    auto window = CreateWindowExW(WS_EX_DLGMODALFRAME | WS_EX_CONTROLPARENT, type.lpszClassName,
                                  L"FPGA details", WS_OVERLAPPEDWINDOW | WS_VISIBLE, CW_USEDEFAULT,
                                  CW_USEDEFAULT, scaled(owner, 760), scaled(owner, 540), owner,
                                  nullptr, type.hInstance, &dialog);
    if (!window)
        fail("Cannot create details window");
    EnableWindow(owner, FALSE);
    SetFocus(dialog.close);
    MSG message{};
    while (IsWindow(window)) {
        const auto status = GetMessageW(&message, nullptr, 0, 0);
        if (status <= 0) {
            if (status == 0)
                PostQuitMessage(static_cast<int>(message.wParam));
            break;
        }
        if (!IsDialogMessageW(window, &message)) {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
    }
    if (IsWindow(window))
        DestroyWindow(window);
    EnableWindow(owner, TRUE);
    SetForegroundWindow(owner);
}
}
