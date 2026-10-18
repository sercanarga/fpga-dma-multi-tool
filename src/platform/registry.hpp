#pragma once
#include "platform/windows.hpp"
#include <optional>
#include <vector>
namespace fpga::win {
class RegistryKey {
    HKEY value_{};

  public:
    RegistryKey(HKEY parent, const std::wstring& path);
    ~RegistryKey();
    RegistryKey(const RegistryKey&) = delete;
    RegistryKey& operator=(const RegistryKey&) = delete;
    std::optional<std::string> string(const wchar_t* name) const;
    std::optional<DWORD> number(const wchar_t* name) const;
    std::vector<std::wstring> children() const;
};
}
