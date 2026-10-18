#include "ui/window.hpp"
#include <algorithm>
#include <shellapi.h>
namespace fpga::ui {
void Window::createControls() {
    tabs_ = control(window_, WC_TABCONTROLW, L"", WS_TABSTOP | WS_CLIPCHILDREN, tabId,
                    WS_EX_CONTROLPARENT);
    for (std::size_t i = 0; i < pageDefinitions.size(); ++i) {
        TCITEMW item{};
        item.mask = TCIF_TEXT;
        item.pszText = const_cast<wchar_t*>(pageDefinitions[i].title);
        SendMessageW(tabs_, TCM_INSERTITEMW, i, reinterpret_cast<LPARAM>(&item));
    }
    for (std::size_t i = 0; i < pages_.size(); ++i) {
        pages_[i] = pageDefinitions[i].create(*this);
        pages_[i]->create(tabs_);
    }
    status_ = control(window_, STATUSCLASSNAMEW, L"Ready",
                      CCS_NORESIZE | CCS_NOPARENTALIGN | WS_CLIPCHILDREN, 0);
    progress_ = control(status_, PROGRESS_CLASSW, L"Working", PBS_MARQUEE, 914);
    ShowWindow(progress_, SW_HIDE);
    credit_ = control(window_, WC_LINK,
                      L"powered by <a href=\"https://github.com/sercanarga\">sercanarga</a>",
                      WS_TABSTOP, creditId);
    font_.update(window_);
    fontHandle_ = font_.get();
    if (options_.operationsEnabled)
        work_ = std::make_unique<WorkQueue>(window_);
    if (options_.persistSettings)
        loadSettings();
    for (auto& page : pages_)
        page->connectionChanged();
    SetWindowPos(credit_, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    SetWindowPos(tabs_, HWND_BOTTOM, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    setBusy(false);
    selectTab(0);
    DragAcceptFiles(window_, TRUE);
    if (!SetTimer(window_, 1, 100, nullptr))
        fail("Cannot initialize worker notification timer");
}
void Window::layout() {
    if (!tabs_)
        return;
    RECT area{};
    GetClientRect(window_, &area);
    const auto s = [&](int value) { return scaled(window_, value); };
    const int height = s(24);
    const int bottom = area.bottom - height, margin = s(10);
    const int current = TabCtrl_GetCurSel(tabs_);
    move(tabs_, margin, margin, area.right - 2 * margin, bottom - 2 * margin);
    const auto dc = GetDC(credit_);
    if (!dc)
        fail("Cannot measure status text");
    const auto previous = SelectObject(dc, fontHandle_);
    SIZE extent{};
    constexpr std::wstring_view creditText = L"powered by sercanarga";
    GetTextExtentPoint32W(dc, creditText.data(), static_cast<int>(creditText.size()), &extent);
    SelectObject(dc, previous);
    ReleaseDC(credit_, dc);
    const int creditWidth = extent.cx + s(8);
    move(status_, 0, bottom, area.right, height);
    const int creditStart = area.right - creditWidth - s(26);
    if (active_) {
        const int parts[]{creditStart - s(144), creditStart, -1};
        SendMessageW(status_, SB_SETPARTS, 3, reinterpret_cast<LPARAM>(parts));
        RECT progressPart{};
        SendMessageW(status_, SB_GETRECT, 1, reinterpret_cast<LPARAM>(&progressPart));
        const int inset = s(3);
        move(progress_, progressPart.left + inset, progressPart.top + inset,
             std::max(0L, progressPart.right - progressPart.left - 2 * inset),
             std::max(0L, progressPart.bottom - progressPart.top - 2 * inset));
    } else {
        const int parts[]{creditStart, -1};
        SendMessageW(status_, SB_SETPARTS, 2, reinterpret_cast<LPARAM>(parts));
    }
    RECT creditPart{};
    SendMessageW(status_, SB_GETRECT, active_ ? 2 : 1, reinterpret_cast<LPARAM>(&creditPart));
    MapWindowPoints(status_, window_, reinterpret_cast<POINT*>(&creditPart), 2);
    const int partHeight = creditPart.bottom - creditPart.top;
    const int creditHeight = std::min(partHeight, static_cast<int>(extent.cy));
    SetWindowPos(credit_, HWND_TOP, area.right - creditWidth - s(18),
                 creditPart.top + (partHeight - creditHeight) / 2, creditWidth, creditHeight,
                 SWP_NOACTIVATE);
    RECT page{};
    GetClientRect(tabs_, &page);
    TabCtrl_AdjustRect(tabs_, FALSE, &page);
    for (std::size_t i = 0; i < pages_.size(); ++i) {
        move(pages_[i]->window(), page.left, page.top, page.right - page.left,
             page.bottom - page.top);
        ShowWindow(pages_[i]->window(), static_cast<int>(i) == current ? SW_SHOW : SW_HIDE);
    }
    RedrawWindow(window_, nullptr, nullptr,
                 RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_FRAME | RDW_UPDATENOW);
}
}
