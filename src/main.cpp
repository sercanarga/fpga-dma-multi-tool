#include "application/worker.hpp"
#include "platform/drivers.hpp"
#include "ui/window.hpp"
#include <shellapi.h>
int WINAPI wWinMain(HINSTANCE, HINSTANCE, PWSTR, int show) {
    try {
        SetDefaultDllDirectories(LOAD_LIBRARY_SEARCH_DEFAULT_DIRS);
        int count = 0;
        auto* raw = CommandLineToArgvW(GetCommandLineW(), &count);
        if (!raw)
            fpga::fail("Cannot read command line");
        std::vector<std::wstring> arguments;
        try {
            for (int i = 0; i < count; ++i)
                arguments.emplace_back(raw[i]);
        } catch (...) {
            LocalFree(raw);
            throw;
        }
        LocalFree(raw);
        if (arguments.size() == 4 && arguments[1] == L"--administrator-file")
            return fpga::runAdministratorFile(arguments[2], fpga::win::utf8(arguments[3]));
        if (arguments.size() == 2 && arguments[1] == L"--worker")
            return fpga::runWorker();
        fpga::win::ComApartment apartment(COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
        return fpga::ui::runGui(show);
    } catch (const std::exception& error) {
        OutputDebugStringA(error.what());
        return 1;
    }
}
