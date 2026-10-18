#include "application/limits.hpp"
#include "application/protocol.hpp"
#include "core/files.hpp"
#include "platform/devices.hpp"
#include "platform/drivers.hpp"
#include <shellapi.h>
namespace fpga {
namespace {
bool elevated() {
    HANDLE raw{};
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &raw))
        fail("Cannot inspect process token: " + win::error());
    win::Handle token(raw);
    TOKEN_ELEVATION elevation{};
    DWORD bytes{};
    if (!GetTokenInformation(token.get(), TokenElevation, &elevation, sizeof(elevation), &bytes))
        fail("Cannot read process elevation: " + win::error());
    return elevation.TokenIsElevated != 0;
}
bool administer(const Request& request, Context& context) {
    request.validate();
    if (!request.privileged())
        fail("Operation is not a Windows administration request");
    if (!elevated())
        fail("Windows administrator permission is required");
    if (const auto* driver = std::get_if<DriverRequest>(&request.operation))
        return changeDriver(*driver, context);
    if (const auto* history = std::get_if<RemoveHistoryRequest>(&request.operation))
        return removeDevices(*history, context);
    rescanDevices();
    return false;
}
}
Notice manageWindows(const Request& request, Context& context) {
    request.validate();
    if (!request.privileged())
        fail("Operation is not a Windows administration request");
    context.check();
    win::ComApartment apartment(COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
    const auto executable = win::executablePath().native();
    win::TemporaryDirectory exchange;
    const auto input = exchange.path() / L"request.json";
    const auto output = exchange.path() / L"result.json";
    const auto encoded = encodeRequest(request);
    win::atomicWrite(input, encoded);
    const auto parameters = L"--administrator-file " + quoteArgument(input.native()) + L" " +
                            quoteArgument(win::wide(win::sha256(input)));
    if (parameters.size() > 16000)
        fail("Administration request is too long");
    SHELLEXECUTEINFOW launch{};
    launch.cbSize = sizeof(launch);
    launch.fMask = SEE_MASK_NOCLOSEPROCESS | SEE_MASK_NOASYNC;
    launch.lpVerb = L"runas";
    launch.lpFile = executable.c_str();
    launch.lpParameters = parameters.c_str();
    launch.nShow = SW_HIDE;
    if (!ShellExecuteExW(&launch))
        fail(GetLastError() == ERROR_CANCELLED
                 ? "Administrator permission was cancelled"
                 : "Cannot start administrator process: " + win::error());
    win::Handle process(launch.hProcess);
    if (!process)
        fail("Administrator process handle is unavailable");
    for (;;) {
        const auto status = WaitForSingleObject(process.get(), 100);
        if (status == WAIT_OBJECT_0)
            break;
        if (status != WAIT_TIMEOUT)
            fail("Cannot wait for administrator process: " + win::error());
        context.check();
    }
    DWORD code{};
    if (!GetExitCodeProcess(process.get(), &code))
        fail("Cannot read administrator result: " + win::error());
    if (code != ERROR_SUCCESS && code != ERROR_SUCCESS_REBOOT_REQUIRED) {
        const auto result =
            std::filesystem::exists(output) ? readFile(output, limits::eventBytes) : std::string{};
        if (!result.empty()) {
            const auto event = decodeEvent(result);
            if (!event.error.empty())
                fail(event.error);
        }
        fail("Windows administration failed (exit " + std::to_string(code) + ").");
    }
    std::string message;
    if (const auto* driver = std::get_if<DriverRequest>(&request.operation))
        message = driver->remove ? "Driver removal completed"
                  : driver->kind == Driver::writer
                      ? "Zadig closed. Refresh status to verify Interface 0."
                      : "Driver installation completed. Reconnect the adapter.";
    else if (const auto* history = std::get_if<RemoveHistoryRequest>(&request.operation))
        message = "Removed " + std::to_string(history->instances.size()) +
                  " disconnected device instance(s)";
    else
        message = "Windows device rescan completed";
    if (code == ERROR_SUCCESS_REBOOT_REQUIRED)
        message += " Restart Windows to finish.";
    return {message};
}
int runAdministratorFile(const std::filesystem::path& path, const std::string& expectedHash) {
    win::Handle input(CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr,
                                  OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr));
    if (!input || std::filesystem::file_size(path) > limits::requestBytes ||
        win::sha256(path) != expectedHash)
        fail("Invalid administrator request file");
    const auto encoded = readFile(path, limits::requestBytes);
    try {
        return runAdministrator(win::wide(encoded));
    } catch (const std::exception& error) {
        win::atomicWrite(path.parent_path() / L"result.json", encodeEvent({{}, error.what(), {}}));
        return ERROR_FUNCTION_FAILED;
    }
}

int runAdministrator(const std::wstring& encodedRequest) {
    const auto request = decodeRequest(win::utf8(encodedRequest));
    Context context{std::chrono::steady_clock::now() + request.timeout, {}, {}};
    return administer(request, context) ? ERROR_SUCCESS_REBOOT_REQUIRED : ERROR_SUCCESS;
}
}
