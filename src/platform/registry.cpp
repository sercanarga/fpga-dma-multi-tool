#include "platform/registry.hpp"
namespace fpga::win {
RegistryKey::RegistryKey(HKEY parent, const std::wstring& path) {
    const auto status = RegOpenKeyExW(parent, path.c_str(), 0, KEY_READ | KEY_WOW64_64KEY, &value_);
    if (status != ERROR_SUCCESS && status != ERROR_FILE_NOT_FOUND && status != ERROR_ACCESS_DENIED)
        fail("Cannot open registry key: " + error(status));
}
RegistryKey::~RegistryKey() {
    if (value_)
        RegCloseKey(value_);
}
std::optional<std::string> RegistryKey::string(const wchar_t* name) const {
    DWORD size{};
    if (!value_ || RegGetValueW(value_, nullptr, name, RRF_RT_REG_SZ, nullptr, nullptr, &size) !=
                       ERROR_SUCCESS)
        return std::nullopt;
    if (size > 65536)
        fail("Registry string exceeds 64 KiB");
    std::wstring value(size / sizeof(wchar_t) + 1, L'\0');
    if (RegGetValueW(value_, nullptr, name, RRF_RT_REG_SZ, nullptr, value.data(), &size) !=
        ERROR_SUCCESS)
        return std::nullopt;
    value.resize(std::wcslen(value.c_str()));
    return utf8(value);
}
std::optional<DWORD> RegistryKey::number(const wchar_t* name) const {
    DWORD value{}, size = sizeof(value);
    if (!value_ || RegGetValueW(value_, nullptr, name, RRF_RT_REG_DWORD, nullptr, &value, &size) !=
                       ERROR_SUCCESS)
        return std::nullopt;
    return value;
}
std::vector<std::wstring> RegistryKey::children() const {
    std::vector<std::wstring> result;
    if (!value_)
        return result;
    for (DWORD index = 0;; ++index) {
        wchar_t name[256]{};
        DWORD size = 256;
        auto status = RegEnumKeyExW(value_, index, name, &size, nullptr, nullptr, nullptr, nullptr);
        if (status == ERROR_NO_MORE_ITEMS)
            break;
        if (status != ERROR_SUCCESS)
            fail("Cannot enumerate registry: " + error(status));
        result.emplace_back(name, size);
    }
    return result;
}
}
