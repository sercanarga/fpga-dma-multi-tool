#include "platform/drivers.hpp"
#include "core/text.hpp"
#include "platform/devices.hpp"
#include <algorithm>
#include <set>
#include <setupapi.h>
namespace fpga {
namespace {
class Inf {
    HINF value_;

  public:
    explicit Inf(const std::filesystem::path& path)
        : value_(SetupOpenInfFileW(path.c_str(), nullptr, INF_STYLE_WIN4, nullptr)) {
        if (value_ == INVALID_HANDLE_VALUE)
            fail("Cannot open driver INF: " + win::error());
    }
    ~Inf() {
        SetupCloseInfFile(value_);
    }
    Inf(const Inf&) = delete;
    Inf& operator=(const Inf&) = delete;
    HINF get() const {
        return value_;
    }
    std::wstring field(const wchar_t* section, const wchar_t* key) const {
        INFCONTEXT line{};
        if (!SetupFindFirstLineW(value_, section, key, &line))
            return {};
        DWORD length{};
        SetupGetStringFieldW(&line, 1, nullptr, 0, &length);
        std::wstring result(length + 1, L'\0');
        if (!SetupGetStringFieldW(&line, 1, result.data(), static_cast<DWORD>(result.size()),
                                  nullptr))
            fail("Cannot read driver INF field: " + win::error());
        result.resize(std::wcslen(result.c_str()));
        return result;
    }
    std::wstring originalName() const {
        DWORD size{};
        SetupGetInfInformationW(value_, INFINFO_INF_SPEC_IS_HINF, nullptr, 0, &size);
        if (!size || size > 1024 * 1024)
            fail("Cannot read INF metadata");
        std::vector<BYTE> buffer(size);
        auto* information = reinterpret_cast<SP_INF_INFORMATION*>(buffer.data());
        if (!SetupGetInfInformationW(value_, INFINFO_INF_SPEC_IS_HINF, information, size, nullptr))
            fail("Cannot read INF metadata: " + win::error());
        SP_ORIGINAL_FILE_INFO_W original{};
        original.cbSize = sizeof(original);
        if (!SetupQueryInfOriginalFileInformationW(information, 0, nullptr, &original))
            fail("Cannot read original INF name: " + win::error());
        return original.OriginalInfName;
    }
};
struct Package {
    std::filesystem::path path;
    std::string original, provider;
};
std::filesystem::path infDirectory() {
    wchar_t directory[MAX_PATH]{};
    if (!GetWindowsDirectoryW(directory, MAX_PATH))
        fail("Cannot locate Windows directory");
    return std::filesystem::path(directory) / L"INF";
}
bool oemInfName(const std::string& name) {
    return name.starts_with("oem") && name.ends_with(".inf") && name.size() > 7 &&
           std::all_of(name.begin() + 3, name.end() - 4,
                       [](char value) { return value >= '0' && value <= '9'; });
}
std::vector<Package> packages(Context& context) {
    std::vector<Package> result;
    for (const auto& entry : std::filesystem::directory_iterator(infDirectory())) {
        context.check();
        const auto name = lower(win::utf8(entry.path().filename().native()));
        if (!oemInfName(name) || !entry.is_regular_file())
            continue;
        Inf inf(entry.path());
        result.push_back({entry.path(), lower(win::utf8(inf.originalName())),
                          lower(win::utf8(inf.field(L"Version", L"Provider")))});
    }
    return result;
}
bool matches(const Package& package, Driver driver) {
    if (driver == Driver::ch347)
        return package.original == "ch341wdm.inf" &&
               (package.provider == "wch.cn" ||
                package.provider.find("winchiphead") != package.provider.npos ||
                package.provider.find("nanjing") != package.provider.npos);
    return driver == Driver::d3xx && package.original == "ftdibus3.inf" &&
           (package.provider == "ftdi" ||
            package.provider.find("future technology devices") != package.provider.npos);
}
bool applyPackage(const std::filesystem::path& path, bool remove) {
    win::Module library(win::systemTool(L"newdev.dll"));
    using Update = BOOL(WINAPI*)(HWND, LPCWSTR, DWORD, PBOOL);
    auto update = library.function<Update>(remove ? "DiUninstallDriverW" : "DiInstallDriverW");
    BOOL restart{};
    if (!update(nullptr, path.c_str(), 0, &restart))
        fail(std::string(remove ? "Driver removal failed: " : "Driver installation failed: ") +
             win::error());
    return restart != FALSE;
}
std::wstring infField(INFCONTEXT line, DWORD field) {
    DWORD length{};
    SetupGetStringFieldW(&line, field, nullptr, 0, &length);
    if (!length || length > 65536)
        fail("Invalid driver INF field length");
    std::wstring value(length, L'\0');
    if (!SetupGetStringFieldW(&line, field, value.data(), length, nullptr))
        fail("Cannot read driver INF field: " + win::error());
    value.resize(std::wcslen(value.c_str()));
    return value;
}
}
void validateWriterPackage(const std::filesystem::path& path) {
    Inf inf(path);
    INFCONTEXT manufacturer{};
    if (!SetupFindFirstLineW(inf.get(), L"Manufacturer", nullptr, &manufacturer))
        fail("Writer INF has no manufacturer section");
    std::set<std::wstring> sections;
    do {
        const auto base = infField(manufacturer, 1);
        sections.insert(base);
        const auto count = SetupGetFieldCount(&manufacturer);
        for (DWORD field = 2; field <= count; ++field)
            sections.insert(base + L"." + infField(manufacturer, field));
    } while (SetupFindNextLine(&manufacturer, &manufacturer));
    bool found = false;
    for (const auto& section : sections) {
        INFCONTEXT model{};
        if (!SetupFindFirstLineW(inf.get(), section.c_str(), nullptr, &model))
            continue;
        do {
            const auto count = SetupGetFieldCount(&model);
            if (count < 2)
                fail("Writer INF contains a model without a hardware ID");
            for (DWORD field = 2; field <= count; ++field) {
                const auto hardware = win::utf8(infField(model, field));
                if (!writerPid(hardware + "\\instance"))
                    fail("Writer INF also targets other hardware. Use Device Manager for this "
                         "package.");
                found = true;
            }
        } while (SetupFindNextLine(&model, &model));
    }
    if (!found)
        fail("Selected driver INF does not identify a supported FTDI writer");
}
TableResult driverStatus(Context& context) {
    const auto installed = packages(context);
    TableResult result{{"Driver", "Installed", "Details"}, {}, "Driver status refreshed"};
    for (const auto& [name, kind] :
         {std::pair{"CH347", Driver::ch347}, {"FTDI D3XX", Driver::d3xx}}) {
        const auto count = std::count_if(installed.begin(), installed.end(),
                                         [&](const auto& item) { return matches(item, kind); });
        result.rows.push_back({name, count ? "Yes" : "No",
                               count ? std::to_string(count) + " driver package(s) installed"
                                     : "Driver package not found"});
    }
    const auto writers = ftdiWriters(context);
    const bool ready =
        std::any_of(writers.begin(), writers.end(), [](const auto& item) { return item.ready; });
    result.rows.push_back({"FTDI writer", ready ? "Yes" : "No",
                           ready ? "Interface A is ready"
                                 : "Connect writer and install WinUSB for Interface 0 in Zadig"});
    return result;
}
bool changeDriver(const DriverRequest& request, Context& context) {
    context.check();
    if (request.kind == Driver::writer) {
        auto writers = ftdiWriters(context);
        std::erase_if(writers, [&](const auto& item) {
            return (!request.serial.empty() && item.serial != request.serial) ||
                   (request.remove && !item.ready);
        });
        if (writers.empty())
            fail("Connect a supported FTDI writer first");
        if (!request.remove) {
            const auto zadig = win::runtimePath("drivers/rs232/zadig-2.9.exe");
            win::verifySignature(zadig, L"Akeo Consulting");
            ProcessOptions options;
            options.executable = zadig;
            options.directory = zadig.parent_path();
            execute(context, options);
            return false;
        }
        if (writers.size() != 1)
            fail("Select exactly one FTDI writer by serial number");
        const auto& selected = writers.front().device;
        const auto inf = lower(selected.inf);
        if (!oemInfName(inf))
            fail("Selected writer has no removable OEM driver package");
        for (const auto& device : windowsDevices(context))
            if (lower(device.inf) == inf && lower(device.instance) != lower(selected.instance))
                fail("Writer package is shared by other devices. Use Device Manager for this "
                     "instance.");
        const auto path = infDirectory() / win::wide(inf);
        validateWriterPackage(path);
        return applyPackage(path, true);
    }
    if (request.remove) {
        const auto installed = packages(context);
        bool found = false, reboot = false;
        for (const auto& item : installed) {
            if (!matches(item, request.kind))
                continue;
            context.check();
            found = true;
            reboot = applyPackage(item.path, true) || reboot;
        }
        if (!found)
            fail("Driver package is not installed");
        return reboot;
    }
    const bool wch = request.kind == Driver::ch347;
    const auto directory =
        win::runtimePath(wch ? "drivers/wch/CH341WDM.INF" : "drivers/ftdi/ftdibus3.Inf")
            .parent_path();
    win::verifySignature(directory / (wch ? L"CH341WDM.CAT" : L"ftdibus3.cat"),
                         L"Microsoft Windows Hardware Compatibility Publisher");
    return applyPackage(directory / (wch ? L"CH341WDM.INF" : L"ftdibus3.Inf"), false);
}
}
