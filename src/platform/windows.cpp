#include "platform/windows.hpp"
#include "application/model.hpp"
#include "core/files.hpp"
#include "core/firmware.hpp"
#include "core/text.hpp"
#include <bcrypt.h>
#include <fstream>
#include <limits>
#include <softpub.h>
#include <vector>
#include <wincrypt.h>
#include <wintrust.h>
namespace fpga::win {
void verifySignature(const std::filesystem::path& file, std::wstring_view publisher) {
    WINTRUST_FILE_INFO info{};
    info.cbStruct = sizeof(info);
    info.pcwszFilePath = file.c_str();
    WINTRUST_DATA data{};
    data.cbStruct = sizeof(data);
    data.dwUIChoice = WTD_UI_NONE;
    data.fdwRevocationChecks = WTD_REVOKE_NONE;
    data.dwUnionChoice = WTD_CHOICE_FILE;
    data.pFile = &info;
    data.dwStateAction = WTD_STATEACTION_VERIFY;
    data.dwProvFlags = WTD_CACHE_ONLY_URL_RETRIEVAL;
    GUID policy = WINTRUST_ACTION_GENERIC_VERIFY_V2;
    const auto status = WinVerifyTrust(nullptr, &policy, &data);
    bool trusted = false;
    if (status == ERROR_SUCCESS) {
        auto* provider = WTHelperProvDataFromStateData(data.hWVTStateData);
        auto* signer = provider ? WTHelperGetProvSignerFromChain(provider, 0, FALSE, 0) : nullptr;
        if (signer && signer->csCertChain) {
            wchar_t name[512]{};
            CertGetNameStringW(signer->pasCertChain[0].pCert, CERT_NAME_SIMPLE_DISPLAY_TYPE, 0,
                               nullptr, name, 512);
            trusted = std::wstring_view(name) == publisher;
        }
    }
    data.dwStateAction = WTD_STATEACTION_CLOSE;
    WinVerifyTrust(nullptr, &policy, &data);
    if (!trusted)
        fail("Invalid signature or unexpected publisher: " + utf8(file.filename().native()));
}
ComApartment::ComApartment(DWORD flags) : status_(CoInitializeEx(nullptr, flags)) {
    if (FAILED(status_) && status_ != RPC_E_CHANGED_MODE)
        fail("Cannot initialize COM: " + error(static_cast<DWORD>(status_)));
}
ComApartment::~ComApartment() {
    if (SUCCEEDED(status_))
        CoUninitialize();
}

std::wstring wide(std::string_view text) {
    if (text.empty())
        return {};
    if (text.size() > INT_MAX)
        fail("Text is too large");
    const int size = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(),
                                         static_cast<int>(text.size()), nullptr, 0);
    if (!size)
        fail("Invalid UTF-8 text");
    std::wstring result(static_cast<std::size_t>(size), L'\0');
    MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()),
                        result.data(), size);
    return result;
}
std::string utf8(std::wstring_view text) {
    if (text.empty())
        return {};
    if (text.size() > INT_MAX)
        fail("Text is too large");
    const int size =
        WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, text.data(),
                            static_cast<int>(text.size()), nullptr, 0, nullptr, nullptr);
    if (!size)
        fail("Invalid Unicode text");
    std::string result(static_cast<std::size_t>(size), '\0');
    WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()),
                        result.data(), size, nullptr, nullptr);
    return result;
}
std::string error(DWORD code) {
    wchar_t buffer[2048]{};
    auto count = FormatMessageW(FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS, nullptr,
                                code, 0, buffer, 2048, nullptr);
    return count ? trim(utf8(std::wstring_view(buffer, count)))
                 : "Windows error " + std::to_string(code);
}
std::filesystem::path executablePath() {
    std::wstring path(32768, L'\0');
    const auto size = GetModuleFileNameW(nullptr, path.data(), static_cast<DWORD>(path.size()));
    if (!size || size >= path.size())
        fail("Cannot locate executable");
    path.resize(size);
    return path;
}
std::filesystem::path runtimePath(const std::filesystem::path& relative) {
    auto path = executablePath().parent_path() / relative;
    if (!std::filesystem::is_regular_file(path))
        fail("Missing runtime file: " + pathText(relative) + ". Extract the complete package.");
    return path;
}
std::filesystem::path systemTool(const std::wstring& name) {
    wchar_t path[MAX_PATH]{};
    auto size = GetSystemDirectoryW(path, MAX_PATH);
    if (!size || size >= MAX_PATH)
        fail("Cannot locate Windows system directory");
    return std::filesystem::path(path) / name;
}
Module::Module(const std::filesystem::path& path) {
    value_ = LoadLibraryExW(std::filesystem::absolute(path).c_str(), nullptr,
                            LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_DEFAULT_DIRS);
    if (!value_)
        fail("Cannot load " + pathText(path.filename()) + ": " + error());
}
Module::~Module() {
    if (value_)
        FreeLibrary(value_);
}
TemporaryDirectory::TemporaryDirectory() {
    GUID id{};
    if (FAILED(CoCreateGuid(&id)))
        fail("Cannot create temporary directory identifier");
    wchar_t text[40]{};
    StringFromGUID2(id, text, 40);
    path_ = std::filesystem::temp_directory_path() / (std::wstring(L"fpga-dma-") + text);
    if (!std::filesystem::create_directory(path_))
        fail("Cannot create temporary directory");
}
TemporaryDirectory::~TemporaryDirectory() {
    std::error_code ignored;
    std::filesystem::remove_all(path_, ignored);
}
std::string sha256(const std::filesystem::path& path) {
    auto data = readFile(path, 32 * 1024 * 1024);
    std::vector<unsigned char> digest(32);
    BCRYPT_ALG_HANDLE algorithm{};
    if (BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0) < 0)
        fail("Cannot open SHA-256 provider");
    const auto status = BCryptHash(algorithm, nullptr, 0, reinterpret_cast<PUCHAR>(data.data()),
                                   static_cast<ULONG>(data.size()), digest.data(),
                                   static_cast<ULONG>(digest.size()));
    BCryptCloseAlgorithmProvider(algorithm, 0);
    if (status < 0)
        fail("Cannot hash firmware");
    std::string result;
    for (auto byte : digest)
        result += hex(byte, 2);
    return lower(result);
}
void atomicWrite(const std::filesystem::path& path, std::string_view bytes) {
    GUID id{};
    if (FAILED(CoCreateGuid(&id)))
        fail("Cannot create output identifier");
    wchar_t text[40]{};
    StringFromGUID2(id, text, 40);
    auto temporary = path;
    temporary += text;
    try {
        Handle file(CreateFileW(temporary.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_NEW,
                                FILE_ATTRIBUTE_NORMAL, nullptr));
        if (!file)
            fail("Cannot create output file: " + error());
        if (bytes.size() > MAXDWORD)
            fail("Output exceeds file limit");
        DWORD written = 0;
        if (!WriteFile(file.get(), bytes.data(), static_cast<DWORD>(bytes.size()), &written,
                       nullptr) ||
            written != bytes.size() || !FlushFileBuffers(file.get()))
            fail("Cannot write output: " + error());
        file.reset();
        if (!MoveFileExW(temporary.c_str(), path.c_str(),
                         MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH))
            fail("Cannot replace output: " + error());
    } catch (...) {
        std::error_code ignored;
        std::filesystem::remove(temporary, ignored);
        throw;
    }
}
}
