#include <algorithm>
#include <string>
#include <windows.h>
int wmain(int argc, wchar_t** argv) {
    int first = 1;
    if (argc > 1 && (std::wstring(argv[1]) == L"-m" || std::wstring(argv[1]) == L"-u"))
        ++first;
    std::wstring path;
    for (int i = first; i < argc; ++i) {
        if (i > first)
            path += L' ';
        path += argv[i];
    }
    std::replace(path.begin(), path.end(), L'\\', L'/');
    path += L'\n';
    const auto size = WideCharToMultiByte(CP_UTF8, 0, path.data(), static_cast<int>(path.size()),
                                          nullptr, 0, nullptr, nullptr);
    if (!size)
        return 1;
    std::string output(static_cast<std::size_t>(size), '\0');
    if (!WideCharToMultiByte(CP_UTF8, 0, path.data(), static_cast<int>(path.size()), output.data(),
                             size, nullptr, nullptr))
        return 1;
    DWORD written = 0;
    return WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), output.data(),
                     static_cast<DWORD>(output.size()), &written, nullptr) &&
                   written == output.size()
               ? 0
               : 1;
}
