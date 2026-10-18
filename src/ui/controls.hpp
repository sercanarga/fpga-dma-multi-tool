#pragma once
#include "platform/windows.hpp"
#include <commctrl.h>
#include <optional>
#include <vector>
namespace fpga::ui {
enum class StatusTone { neutral, positive, negative };
std::optional<COLORREF> statusColor(StatusTone tone, COLORREF background);
int scaled(HWND window, int value);
void appendText(HWND window, const std::string& message);
LRESULT controlColors(UINT message, WPARAM dc, LPARAM control);
void refreshSystemColors(HWND window);
void applicationIcons(HWND window);
HWND control(HWND parent, const wchar_t* type, const std::wstring& text, DWORD style, int id,
             DWORD extended = 0);
std::wstring text(HWND window);
void text(HWND window, const std::wstring& value);
int selection(HWND combo);
void select(HWND combo, int index);
void choices(HWND combo, const std::vector<std::wstring>& values);
bool checked(HWND box);
void check(HWND box, bool value);
void move(HWND window, int x, int y, int width, int height);
void clipboard(HWND owner, const std::wstring& text);
std::optional<std::filesystem::path> chooseFile(HWND owner, bool save = false);
bool confirm(HWND owner, const std::wstring& message);
class Font {
    HFONT value_ = nullptr;

  public:
    ~Font() {
        if (value_)
            DeleteObject(value_);
    }
    Font() = default;
    Font(const Font&) = delete;
    Font& operator=(const Font&) = delete;
    void update(HWND owner, bool heading = false);
    HFONT get() const {
        return value_;
    }
};
}
