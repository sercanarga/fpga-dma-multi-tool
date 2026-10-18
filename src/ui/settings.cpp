#include "ui/window.hpp"
#include <shlobj.h>
namespace fpga::ui {
void Window::loadSettings() {
    PWSTR directory{};
    if (FAILED(SHGetKnownFolderPath(FOLDERID_LocalAppData, KF_FLAG_CREATE, nullptr, &directory)))
        return;
    const auto root = std::filesystem::path(directory) / L"FPGA DMA Multi Tool";
    CoTaskMemFree(directory);
    std::error_code error;
    std::filesystem::create_directories(root, error);
    if (error)
        return;
    settingsPath_ = root / L"settings.ini";
    const auto read = [&](const wchar_t* key) {
        std::wstring value(4096, L'\0');
        value.resize(GetPrivateProfileStringW(L"Writer", key, L"", value.data(),
                                              static_cast<DWORD>(value.size()),
                                              settingsPath_.c_str()));
        return value;
    };
    const auto backend = GetPrivateProfileIntW(L"Writer", L"Backend", 0, settingsPath_.c_str()),
               adapter = GetPrivateProfileIntW(L"Writer", L"Adapter", 0, settingsPath_.c_str());
    connection_.backend = static_cast<Backend>(backend <= 2 ? backend : 0);
    connection_.adapter = (adapter <= 16 ? static_cast<int>(adapter) : 0) - 1;
    connection_.serial = win::utf8(read(L"Serial"));
}
void Window::saveSettings() {
    if (settingsPath_.empty())
        return;
    const auto write = [&](const wchar_t* section, const wchar_t* key, const std::wstring& value) {
        if (!WritePrivateProfileStringW(section, key, value.c_str(), settingsPath_.c_str()))
            fail("Cannot save settings: " + win::error());
    };
    write(L"Writer", L"Backend", std::to_wstring(static_cast<int>(connection_.backend)));
    write(L"Writer", L"Adapter", std::to_wstring(connection_.adapter + 1));
    write(L"Writer", L"Serial", win::wide(connection_.serial));
}
}
