#pragma once
#include "core/error.hpp"
#include <cstring>
#include <filesystem>
#include <objbase.h>
#include <string>
#include <utility>
#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>
namespace fpga::win {
std::wstring wide(std::string_view text);
std::string utf8(std::wstring_view text);
std::string error(DWORD code = GetLastError());
std::filesystem::path executablePath();
std::filesystem::path runtimePath(const std::filesystem::path& relative);
std::filesystem::path systemTool(const std::wstring& name);
class ComApartment {
    HRESULT status_;

  public:
    explicit ComApartment(DWORD flags = COINIT_MULTITHREADED);
    ~ComApartment();
    ComApartment(const ComApartment&) = delete;
    ComApartment& operator=(const ComApartment&) = delete;
};
void verifySignature(const std::filesystem::path& file, std::wstring_view publisher);
class Handle {
    HANDLE value_ = nullptr;

  public:
    Handle() = default;
    explicit Handle(HANDLE value) : value_(value == INVALID_HANDLE_VALUE ? nullptr : value) {}
    ~Handle() {
        reset();
    }
    Handle(const Handle&) = delete;
    Handle& operator=(const Handle&) = delete;
    Handle(Handle&& other) noexcept : value_(std::exchange(other.value_, nullptr)) {}
    Handle& operator=(Handle&& other) noexcept {
        if (this != &other) {
            reset();
            value_ = std::exchange(other.value_, nullptr);
        }
        return *this;
    }
    HANDLE get() const noexcept {
        return value_;
    }
    explicit operator bool() const noexcept {
        return value_ != nullptr;
    }
    void reset() noexcept {
        if (value_)
            CloseHandle(std::exchange(value_, nullptr));
    }
};
class Module {
    HMODULE value_ = nullptr;

  public:
    explicit Module(const std::filesystem::path& path);
    ~Module();
    Module(const Module&) = delete;
    Module& operator=(const Module&) = delete;
    template <class T> T function(const char* name) const {
        const auto address = GetProcAddress(value_, name);
        if (!address)
            fail(std::string("Missing runtime function: ") + name);
        T value{};
        static_assert(sizeof(value) == sizeof(address));
        std::memcpy(&value, &address, sizeof(value));
        return value;
    }
};
class TemporaryDirectory {
    std::filesystem::path path_;

  public:
    TemporaryDirectory();
    ~TemporaryDirectory();
    TemporaryDirectory(const TemporaryDirectory&) = delete;
    TemporaryDirectory& operator=(const TemporaryDirectory&) = delete;
    const std::filesystem::path& path() const {
        return path_;
    }
};
std::string sha256(const std::filesystem::path& path);
void atomicWrite(const std::filesystem::path& path, std::string_view bytes);
}
