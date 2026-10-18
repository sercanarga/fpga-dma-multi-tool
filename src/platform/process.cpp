#include "platform/process.hpp"
#include "application/limits.hpp"
#include <algorithm>
#include <thread>
namespace fpga {
void Context::check() const {
    if (stop.stop_requested())
        fail("Operation cancelled");
    if (std::chrono::steady_clock::now() >= deadline)
        fail("Operation timed out");
}
std::wstring quoteArgument(std::wstring_view argument) {
    std::wstring result = L"\"";
    std::size_t slashes = 0;
    for (wchar_t c : argument) {
        if (c == L'\\') {
            ++slashes;
            continue;
        }
        result.append(slashes * (c == L'"' ? 2 : 1), L'\\');
        slashes = 0;
        if (c == L'"')
            result += L'\\';
        result += c;
    }
    result.append(slashes * 2, L'\\');
    return result + L'"';
}
namespace {
struct EnvironmentLess {
    bool operator()(const std::wstring& a, const std::wstring& b) const {
        return CompareStringOrdinal(a.c_str(), -1, b.c_str(), -1, TRUE) == CSTR_LESS_THAN;
    }
};
std::vector<wchar_t> environmentBlock(const Environment& overrides) {
    auto* raw = GetEnvironmentStringsW();
    if (!raw)
        fail("Cannot read process environment");
    std::map<std::wstring, std::wstring, EnvironmentLess> values;
    try {
        for (auto* p = raw; *p; p += wcslen(p) + 1) {
            std::wstring item(p);
            auto pos = item.find(L'=', item.front() == L'=' ? 1 : 0);
            if (pos != item.npos)
                values[item.substr(0, pos)] = item.substr(pos + 1);
        }
    } catch (...) {
        FreeEnvironmentStringsW(raw);
        throw;
    }
    FreeEnvironmentStringsW(raw);
    for (const auto& [key, value] : overrides)
        values[key] = value;
    std::vector<wchar_t> result;
    for (const auto& [key, value] : values) {
        auto item = key + L'=' + value;
        result.insert(result.end(), item.begin(), item.end());
        result.push_back(0);
    }
    result.push_back(0);
    return result;
}
class Attributes {
    std::vector<std::byte> buffer_;

  public:
    explicit Attributes(std::span<HANDLE> handles) {
        SIZE_T size = 0;
        InitializeProcThreadAttributeList(nullptr, 1, 0, &size);
        buffer_.resize(size);
        if (!InitializeProcThreadAttributeList(get(), 1, 0, &size))
            fail("Cannot initialize process attributes");
        if (!UpdateProcThreadAttribute(get(), 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, handles.data(),
                                       handles.size_bytes(), nullptr, nullptr)) {
            DeleteProcThreadAttributeList(get());
            buffer_.clear();
            fail("Cannot restrict inherited handles");
        }
    }
    Attributes(const Attributes&) = delete;
    Attributes& operator=(const Attributes&) = delete;
    ~Attributes() {
        if (!buffer_.empty())
            DeleteProcThreadAttributeList(get());
    }
    LPPROC_THREAD_ATTRIBUTE_LIST get() {
        return reinterpret_cast<LPPROC_THREAD_ATTRIBUTE_LIST>(buffer_.data());
    }
};
}
ProcessResult execute(Context& context, const ProcessOptions& options) {
    context.check();
    if (options.input.size() > limits::requestBytes)
        fail("Process input exceeds limit");
    SECURITY_ATTRIBUTES security{sizeof(security), nullptr, TRUE};
    HANDLE inRead{}, inWrite{}, outRead{}, outWrite{};
    if (!CreatePipe(&inRead, &inWrite, &security, 65536))
        fail("Cannot create input pipe");
    win::Handle inputRead(inRead), inputWrite(inWrite);
    if (!CreatePipe(&outRead, &outWrite, &security, 0))
        fail("Cannot create output pipe");
    win::Handle outputRead(outRead), outputWrite(outWrite);
    if (!SetHandleInformation(inputWrite.get(), HANDLE_FLAG_INHERIT, 0) ||
        !SetHandleInformation(outputRead.get(), HANDLE_FLAG_INHERIT, 0))
        fail("Cannot isolate process pipes");
    win::Handle job(CreateJobObjectW(nullptr, nullptr));
    if (!job)
        fail("Cannot create process job");
    JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
    limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if (!SetInformationJobObject(job.get(), JobObjectExtendedLimitInformation, &limits,
                                 sizeof(limits)))
        fail("Cannot configure process job");
    HANDLE inherited[]{inputRead.get(), outputWrite.get()};
    Attributes attributes(inherited);
    STARTUPINFOEXW startup{};
    startup.StartupInfo.cb = sizeof(startup);
    startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    startup.StartupInfo.hStdInput = inputRead.get();
    startup.StartupInfo.hStdOutput = outputWrite.get();
    startup.StartupInfo.hStdError = outputWrite.get();
    startup.lpAttributeList = attributes.get();
    auto command = quoteArgument(options.executable.native());
    for (const auto& argument : options.arguments)
        command += L' ' + quoteArgument(argument);
    auto environment = environmentBlock(options.environment);
    PROCESS_INFORMATION info{};
    if (!CreateProcessW(options.executable.c_str(), command.data(), nullptr, nullptr, TRUE,
                        CREATE_SUSPENDED | CREATE_NO_WINDOW | CREATE_UNICODE_ENVIRONMENT |
                            EXTENDED_STARTUPINFO_PRESENT,
                        environment.data(),
                        options.directory.empty() ? nullptr : options.directory.c_str(),
                        &startup.StartupInfo, &info))
        fail("Cannot start process: " + win::error());
    win::Handle process(info.hProcess), thread(info.hThread);
    if (!AssignProcessToJobObject(job.get(), process.get())) {
        const auto error = GetLastError();
        TerminateProcess(process.get(), 1);
        WaitForSingleObject(process.get(), 5000);
        fail("Cannot attach child to process job: " + win::error(error));
    }
    if (ResumeThread(thread.get()) == static_cast<DWORD>(-1))
        fail("Cannot resume child process");
    inputRead.reset();
    outputWrite.reset();
    thread.reset();
    if (!options.input.empty()) {
        DWORD written = 0;
        if (!WriteFile(inputWrite.get(), options.input.data(),
                       static_cast<DWORD>(options.input.size()), &written, nullptr) ||
            written != options.input.size())
            fail("Cannot send worker request");
    }
    inputWrite.reset();
    ProcessResult result{};
    const auto drain = [&] {
        for (;;) {
            context.check();
            DWORD available = 0;
            if (!PeekNamedPipe(outputRead.get(), nullptr, 0, nullptr, &available, nullptr)) {
                if (GetLastError() == ERROR_BROKEN_PIPE)
                    return;
                fail("Cannot read process output");
            }
            if (!available)
                return;
            char buffer[8192];
            DWORD count = 0;
            if (!ReadFile(outputRead.get(), buffer, std::min<DWORD>(available, sizeof(buffer)),
                          &count, nullptr))
                fail("Cannot read process output");
            std::string chunk(buffer, count);
            if (options.captureOutput) {
                if (count > limits::diagnosticBytes - result.output.size())
                    fail("Process output exceeds 16 MiB");
                result.output += chunk;
            }
            if (context.log)
                context.log(chunk);
        }
    };
    for (;;) {
        context.check();
        drain();
        auto state = WaitForSingleObject(process.get(), 10);
        if (state == WAIT_OBJECT_0)
            break;
        if (state == WAIT_FAILED)
            fail("Cannot wait for child process");
    }
    drain();
    if (!GetExitCodeProcess(process.get(), &result.exitCode))
        fail("Cannot read process exit code");
    if (result.exitCode && !options.allowFailure)
        fail("Process failed (" + std::to_string(result.exitCode) + "): " + result.output);
    return result;
}
}
