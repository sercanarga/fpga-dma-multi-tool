#include "ui/controls.hpp"
#include "application/limits.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <shobjidl.h>
#include <wrl/client.h>
namespace fpga::ui {
std::optional<COLORREF> statusColor(StatusTone tone, COLORREF background) {
    HIGHCONTRASTW contrast{sizeof(contrast), 0, nullptr};
    if (tone == StatusTone::neutral ||
        !SystemParametersInfoW(SPI_GETHIGHCONTRAST, sizeof(contrast), &contrast, 0) ||
        (contrast.dwFlags & HCF_HIGHCONTRASTON))
        return std::nullopt;
    const bool dark =
        299 * GetRValue(background) + 587 * GetGValue(background) + 114 * GetBValue(background) <
        128000;
    if (tone == StatusTone::positive)
        return dark ? RGB(74, 222, 128) : RGB(21, 128, 61);
    return dark ? RGB(248, 113, 113) : RGB(185, 28, 28);
}
int scaled(HWND window, int value) {
    return MulDiv(value, static_cast<int>(GetDpiForWindow(window)), 96);
}
void appendText(HWND window, const std::string& message) {
    if (!window)
        return;
    const auto value = win::wide(message);
    std::wstring normalized;
    normalized.reserve(value.size());
    for (std::size_t i = 0; i < value.size(); ++i) {
        if (value[i] == L'\n' && (i == 0 || value[i - 1] != L'\r'))
            normalized += L'\r';
        normalized += value[i];
    }
    const auto current = static_cast<std::size_t>(GetWindowTextLengthW(window));
    constexpr std::size_t limit = limits::diagnosticBytes;
    if (current + normalized.size() > limit) {
        constexpr std::wstring_view notice = L"[Earlier output removed: display limit]\r\n";
        const auto available = limit - notice.size();
        if (normalized.size() >= available) {
            auto offset = normalized.size() - available;
            if (offset && normalized[offset] >= 0xdc00 && normalized[offset] <= 0xdfff)
                ++offset;
            normalized.erase(0, offset);
            normalized.insert(0, notice);
            text(window, L"");
        } else {
            const auto removed = current + normalized.size() - available;
            SendMessageW(window, EM_SETSEL, 0, static_cast<LPARAM>(removed));
            SendMessageW(window, EM_REPLACESEL, FALSE, reinterpret_cast<LPARAM>(L""));
            SendMessageW(window, EM_SETSEL, 0, 0);
            SendMessageW(window, EM_REPLACESEL, FALSE, reinterpret_cast<LPARAM>(notice.data()));
        }
    }
    const auto end = GetWindowTextLengthW(window);
    SendMessageW(window, EM_SETSEL, static_cast<WPARAM>(end), static_cast<LPARAM>(end));
    SendMessageW(window, EM_REPLACESEL, FALSE, reinterpret_cast<LPARAM>(normalized.c_str()));
}
LRESULT controlColors(UINT message, WPARAM dc, LPARAM control) {
    const auto child = reinterpret_cast<HWND>(control);
    const auto context = reinterpret_cast<HDC>(dc);
    wchar_t type[32]{};
    GetClassNameW(child, type, static_cast<int>(std::size(type)));
    const bool field = message == WM_CTLCOLOREDIT ||
                       (message == WM_CTLCOLORSTATIC && lstrcmpiW(type, L"Edit") == 0);
    const bool enabled = IsWindowEnabled(child);
    const int background = field && enabled ? COLOR_WINDOW : COLOR_BTNFACE;
    SetTextColor(context, GetSysColor(!enabled ? COLOR_GRAYTEXT
                                      : field  ? COLOR_WINDOWTEXT
                                               : COLOR_BTNTEXT));
    SetBkColor(context, GetSysColor(background));
    return reinterpret_cast<LRESULT>(GetSysColorBrush(background));
}
void applicationIcons(HWND window) {
    const auto dpi = GetDpiForWindow(window);
    for (const auto type : {ICON_SMALL, ICON_BIG}) {
        const auto width =
            GetSystemMetricsForDpi(type == ICON_SMALL ? SM_CXSMICON : SM_CXICON, dpi);
        const auto height =
            GetSystemMetricsForDpi(type == ICON_SMALL ? SM_CYSMICON : SM_CYICON, dpi);
        const auto icon = LoadImageW(GetModuleHandleW(nullptr), MAKEINTRESOURCEW(1), IMAGE_ICON,
                                     width, height, LR_SHARED);
        if (!icon)
            fail("Cannot load application icon");
        SendMessageW(window, WM_SETICON, type, reinterpret_cast<LPARAM>(icon));
    }
}
void refreshSystemColors(HWND window) {
    EnumChildWindows(
        window,
        [](HWND child, LPARAM) -> BOOL {
            SendMessageW(child, WM_SYSCOLORCHANGE, 0, 0);
            return TRUE;
        },
        0);
    RedrawWindow(window, nullptr, nullptr,
                 RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_FRAME);
}
HWND control(HWND parent, const wchar_t* type, const std::wstring& title, DWORD style, int id,
             DWORD extended) {
    auto window = CreateWindowExW(extended, type, title.c_str(),
                                  WS_CHILD | WS_VISIBLE | WS_CLIPSIBLINGS | style, 0, 0, 0, 0,
                                  parent, reinterpret_cast<HMENU>(static_cast<INT_PTR>(id)),
                                  GetModuleHandleW(nullptr), nullptr);
    if (!window)
        fail("Cannot create Windows control: " + win::error());
    SendMessageW(window, WM_SETFONT, SendMessageW(parent, WM_GETFONT, 0, 0), TRUE);
    return window;
}
std::wstring text(HWND window) {
    const int length = GetWindowTextLengthW(window);
    std::wstring value(static_cast<std::size_t>(length) + 1, L'\0');
    auto copied = GetWindowTextW(window, value.data(), length + 1);
    value.resize(static_cast<std::size_t>(copied));
    return value;
}
void text(HWND window, const std::wstring& value) {
    SetWindowTextW(window, value.c_str());
}
int selection(HWND combo) {
    return static_cast<int>(SendMessageW(combo, CB_GETCURSEL, 0, 0));
}
void select(HWND combo, int index) {
    SendMessageW(combo, CB_SETCURSEL, static_cast<WPARAM>(index), 0);
}
void choices(HWND combo, const std::vector<std::wstring>& values) {
    for (const auto& value : values)
        SendMessageW(combo, CB_ADDSTRING, 0, reinterpret_cast<LPARAM>(value.c_str()));
    select(combo, 0);
}
bool checked(HWND box) {
    return SendMessageW(box, BM_GETCHECK, 0, 0) == BST_CHECKED;
}
void check(HWND box, bool value) {
    SendMessageW(box, BM_SETCHECK, value ? BST_CHECKED : BST_UNCHECKED, 0);
}
void move(HWND window, int x, int y, int width, int height) {
    MoveWindow(window, x, y, std::max(0, width), std::max(0, height), TRUE);
}
bool confirm(HWND owner, const std::wstring& message) {
    return MessageBoxW(owner, message.c_str(), L"FPGA DMA Multi Tool",
                       MB_YESNO | MB_ICONWARNING | MB_DEFBUTTON2) == IDYES;
}
void clipboard(HWND owner, const std::wstring& value) {
    if (!OpenClipboard(owner))
        fail("Cannot open clipboard");
    const auto bytes = (value.size() + 1) * sizeof(wchar_t);
    HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, bytes);
    if (!memory) {
        CloseClipboard();
        fail("Cannot allocate clipboard data");
    }
    void* pointer = GlobalLock(memory);
    if (!pointer) {
        GlobalFree(memory);
        CloseClipboard();
        fail("Cannot lock clipboard data");
    }
    std::memcpy(pointer, value.c_str(), bytes);
    GlobalUnlock(memory);
    if (!EmptyClipboard() || !SetClipboardData(CF_UNICODETEXT, memory)) {
        GlobalFree(memory);
        CloseClipboard();
        fail("Cannot write clipboard");
    }
    CloseClipboard();
}
std::optional<std::filesystem::path> chooseFile(HWND owner, bool save) {
    Microsoft::WRL::ComPtr<IFileDialog> dialog;
    auto result = CoCreateInstance(save ? CLSID_FileSaveDialog : CLSID_FileOpenDialog, nullptr,
                                   CLSCTX_INPROC_SERVER, IID_PPV_ARGS(dialog.GetAddressOf()));
    if (FAILED(result))
        fail("Cannot open file dialog");
    DWORD options{};
    dialog->GetOptions(&options);
    dialog->SetOptions(options | FOS_FORCEFILESYSTEM |
                       (save ? FOS_OVERWRITEPROMPT : FOS_FILEMUSTEXIST));
    const COMDLG_FILTERSPEC firmware[]{{L"FPGA firmware (*.bit;*.bin)", L"*.bit;*.bin"},
                                       {L"All files (*.*)", L"*.*"}};
    const COMDLG_FILTERSPEC json[]{{L"JSON report (*.json)", L"*.json"}};
    dialog->SetFileTypes(save ? 1 : 2, save ? json : firmware);
    if (save) {
        dialog->SetDefaultExtension(L"json");
        dialog->SetFileName(L"fpga-report.json");
    }
    result = dialog->Show(owner);
    if (result == HRESULT_FROM_WIN32(ERROR_CANCELLED))
        return std::nullopt;
    if (FAILED(result))
        fail("File dialog failed");
    Microsoft::WRL::ComPtr<IShellItem> item;
    if (FAILED(dialog->GetResult(item.GetAddressOf())))
        fail("Cannot read selected file");
    PWSTR path{};
    if (FAILED(item->GetDisplayName(SIGDN_FILESYSPATH, &path)))
        fail("Cannot read selected path");
    std::filesystem::path selected(path);
    CoTaskMemFree(path);
    return selected;
}
void Font::update(HWND owner, bool heading) {
    NONCLIENTMETRICSW metrics{};
    metrics.cbSize = sizeof(metrics);
    if (!SystemParametersInfoForDpi(SPI_GETNONCLIENTMETRICS, sizeof(metrics), &metrics, 0,
                                    GetDpiForWindow(owner)))
        fail("Cannot read Windows UI font");
    if (heading) {
        metrics.lfMessageFont.lfWeight = FW_SEMIBOLD;
        metrics.lfMessageFont.lfHeight = MulDiv(metrics.lfMessageFont.lfHeight, 5, 4);
    }
    auto font = CreateFontIndirectW(&metrics.lfMessageFont);
    if (!font)
        fail("Cannot create Windows UI font");
    auto old = value_;
    value_ = font;
    SendMessageW(owner, WM_SETFONT, reinterpret_cast<WPARAM>(font), TRUE);
    EnumChildWindows(
        owner,
        [](HWND child, LPARAM data) -> BOOL {
            SendMessageW(child, WM_SETFONT, static_cast<WPARAM>(data), TRUE);
            return TRUE;
        },
        reinterpret_cast<LPARAM>(font));
    if (old)
        DeleteObject(old);
}
}
