#define INITGUID
#include "platform/devices.hpp"
#include "application/history.hpp"
#include "core/text.hpp"
#include "platform/registry.hpp"
#include <algorithm>
#include <cfgmgr32.h>
#include <devpkey.h>
#include <pciprop.h>
#include <set>
#include <setupapi.h>
#include <tuple>
namespace fpga {
namespace {
class DeviceSet {
    HDEVINFO value_;

  public:
    explicit DeviceSet(bool presentOnly)
        : value_(SetupDiGetClassDevsW(nullptr, nullptr, nullptr,
                                      DIGCF_ALLCLASSES | (presentOnly ? DIGCF_PRESENT : 0))) {
        if (value_ == INVALID_HANDLE_VALUE)
            fail("Cannot enumerate Windows devices: " + win::error());
    }
    ~DeviceSet() {
        SetupDiDestroyDeviceInfoList(value_);
    }
    DeviceSet(const DeviceSet&) = delete;
    DeviceSet& operator=(const DeviceSet&) = delete;
    HDEVINFO get() const {
        return value_;
    }
};
std::string property(HDEVINFO set, SP_DEVINFO_DATA& device, const DEVPROPKEY& key) {
    DEVPROPTYPE type{};
    DWORD bytes{};
    if (!SetupDiGetDevicePropertyW(set, &device, &key, &type, nullptr, 0, &bytes, 0) &&
        GetLastError() != ERROR_INSUFFICIENT_BUFFER)
        return {};
    if (type != DEVPROP_TYPE_STRING || bytes > 65536)
        return {};
    std::wstring value(bytes / sizeof(wchar_t) + 1, L'\0');
    if (!SetupDiGetDevicePropertyW(set, &device, &key, &type, reinterpret_cast<BYTE*>(value.data()),
                                   bytes, nullptr, 0))
        return {};
    return win::utf8(value.c_str());
}
std::optional<std::uint32_t> numberProperty(HDEVINFO set, SP_DEVINFO_DATA& device,
                                            const DEVPROPKEY& key) {
    DEVPROPTYPE type{};
    std::uint32_t value{};
    if (!SetupDiGetDevicePropertyW(set, &device, &key, &type, reinterpret_cast<BYTE*>(&value),
                                   sizeof(value), nullptr, 0) ||
        type != DEVPROP_TYPE_UINT32)
        return std::nullopt;
    return value;
}
std::string instanceId(HDEVINFO set, SP_DEVINFO_DATA& device) {
    wchar_t id[MAX_DEVICE_ID_LEN]{};
    if (!SetupDiGetDeviceInstanceIdW(set, &device, id, MAX_DEVICE_ID_LEN, nullptr))
        fail("Cannot read device instance: " + win::error());
    return win::utf8(id);
}
bool present(HDEVINFO set, SP_DEVINFO_DATA& device) {
    DEVPROPTYPE type{};
    DEVPROP_BOOLEAN value{};
    if (!SetupDiGetDevicePropertyW(set, &device, &DEVPKEY_Device_IsPresent, &type,
                                   reinterpret_cast<BYTE*>(&value), sizeof(value), nullptr, 0) ||
        type != DEVPROP_TYPE_BOOLEAN)
        fail("Cannot determine device presence: " + win::error());
    return value == DEVPROP_TRUE;
}
}
std::vector<WindowsDevice> windowsDevices(Context& context, bool presentOnly) {
    DeviceSet set(presentOnly);
    std::vector<WindowsDevice> result;
    for (DWORD index = 0;; ++index) {
        context.check();
        SP_DEVINFO_DATA data{};
        data.cbSize = sizeof(data);
        if (!SetupDiEnumDeviceInfo(set.get(), index, &data)) {
            if (GetLastError() != ERROR_NO_MORE_ITEMS)
                fail("Device enumeration failed: " + win::error());
            break;
        }
        WindowsDevice device;
        device.instance = instanceId(set.get(), data);
        device.name = property(set.get(), data, DEVPKEY_Device_FriendlyName);
        if (device.name.empty())
            device.name = property(set.get(), data, DEVPKEY_Device_DeviceDesc);
        if (device.name.empty())
            device.name = device.instance;
        device.deviceClass = property(set.get(), data, DEVPKEY_Device_Class);
        device.enumerator = property(set.get(), data, DEVPKEY_Device_EnumeratorName);
        device.manufacturer = property(set.get(), data, DEVPKEY_Device_Manufacturer);
        device.service = property(set.get(), data, DEVPKEY_Device_Service);
        device.inf = property(set.get(), data, DEVPKEY_Device_DriverInfPath);
        device.present = presentOnly || present(set.get(), data);
        if (device.instance.starts_with("PCI\\") && device.present) {
            device.currentWidth =
                numberProperty(set.get(), data, DEVPKEY_PciDevice_CurrentLinkWidth);
            device.maximumWidth = numberProperty(set.get(), data, DEVPKEY_PciDevice_MaxLinkWidth);
        }
        result.push_back(std::move(device));
    }
    return result;
}
std::optional<std::string> writerPid(std::string_view instance) {
    auto id = lower(std::string(instance));
    if (!id.starts_with("usb\\vid_0403&pid_"))
        return std::nullopt;
    const auto suffix = id.find('\\', 4);
    if (suffix == std::string::npos || suffix + 1 == id.size())
        return std::nullopt;
    auto hardware = id.substr(4, suffix - 4);
    if (hardware.size() != 17 && hardware.size() != 23)
        return std::nullopt;
    if (hardware.size() == 23 && !hardware.ends_with("&mi_00"))
        return std::nullopt;
    auto pid = hardware.substr(13, 4);
    if (pid != "6010" && pid != "6011" && pid != "6014")
        return std::nullopt;
    return pid;
}
std::vector<FtdiWriter> ftdiWriters(Context& context) {
    std::vector<FtdiWriter> result;
    for (auto& device : windowsDevices(context, true)) {
        const auto pid = writerPid(device.instance);
        if (!pid)
            continue;
        const auto service = lower(device.service);
        const auto serial = device.instance.substr(device.instance.rfind('\\') + 1);
        const bool ready = service == "winusb" || service == "libusbk" || service == "libusb0";
        result.push_back({std::move(device), *pid, serial, ready});
    }
    return result;
}
HistoryResult deviceHistory(Context& context) {
    HistoryResult result;
    std::set<std::string> seen;
    for (const auto& device : windowsDevices(context)) {
        result.devices.push_back(
            {device.instance, device.name, device.deviceClass, device.enumerator, device.present});
        seen.insert(lower(device.instance));
    }
    const std::wstring root = L"SYSTEM\\CurrentControlSet\\Enum";
    for (const auto& enumerator : win::RegistryKey(HKEY_LOCAL_MACHINE, root).children()) {
        const auto branch = root + L"\\" + enumerator;
        for (const auto& device : win::RegistryKey(HKEY_LOCAL_MACHINE, branch).children()) {
            const auto path = branch + L"\\" + device;
            for (const auto& instance : win::RegistryKey(HKEY_LOCAL_MACHINE, path).children()) {
                context.check();
                const auto id = win::utf8(enumerator + L"\\" + device + L"\\" + instance);
                if (!seen.insert(lower(id)).second)
                    continue;
                win::RegistryKey key(HKEY_LOCAL_MACHINE, path + L"\\" + instance);
                auto name =
                    key.string(L"FriendlyName").value_or(key.string(L"DeviceDesc").value_or(id));
                if (name.starts_with('@') && name.find(';') != name.npos)
                    name.erase(0, name.find(';') + 1);
                result.devices.push_back(
                    {id, name, key.string(L"Class").value_or(""), win::utf8(enumerator), false});
            }
        }
    }
    std::sort(result.devices.begin(), result.devices.end(), [](const auto& a, const auto& b) {
        return std::tie(a.deviceClass, a.name, a.instance) <
               std::tie(b.deviceClass, b.name, b.instance);
    });
    return result;
}
namespace {
bool removeDevice(const std::string& instance, Context& context) {
    context.check();
    DeviceSet set(false);
    SP_DEVINFO_DATA device{};
    device.cbSize = sizeof(device);
    if (!SetupDiOpenDeviceInfoW(set.get(), win::wide(instance).c_str(), nullptr, 0, &device))
        fail("Cannot open device instance: " + win::error());
    if (present(set.get(), device))
        fail("Disconnect the device before removing its history");
    SP_REMOVEDEVICE_PARAMS parameters{};
    parameters.ClassInstallHeader.cbSize = sizeof(SP_CLASSINSTALL_HEADER);
    parameters.ClassInstallHeader.InstallFunction = DIF_REMOVE;
    parameters.Scope = DI_REMOVEDEVICE_GLOBAL;
    if (!SetupDiSetClassInstallParamsW(set.get(), &device, &parameters.ClassInstallHeader,
                                       sizeof(parameters)) ||
        !SetupDiCallClassInstaller(DIF_REMOVE, set.get(), &device))
        fail("Device removal failed: " + win::error());
    SP_DEVINSTALL_PARAMS_W status{};
    status.cbSize = sizeof(status);
    if (!SetupDiGetDeviceInstallParamsW(set.get(), &device, &status))
        fail("Cannot read device removal status: " + win::error());
    return (status.Flags & (DI_NEEDREBOOT | DI_NEEDRESTART)) != 0;
}
}
bool removeDevices(const RemoveHistoryRequest& request, Context& context) {
    validateHistoryRemoval(request, deviceHistory(context).devices);
    auto report = removeHistoryInstances(
        request, [&](const auto& instance) { return removeDevice(instance, context); },
        [&] { context.check(); });
    if (!report.rebootRequired && !report.removed.empty()) {
        try {
            std::set<std::string> remaining;
            for (const auto& device : deviceHistory(context).devices)
                remaining.insert(lower(device.instance));
            std::erase_if(report.removed, [&](const auto& instance) {
                if (!remaining.contains(lower(instance)))
                    return false;
                report.failures.push_back(
                    instance + ": Windows still reports this instance. Rescan or restart Windows.");
                return true;
            });
        } catch (const std::exception& error) {
            report.failures.push_back(std::string("Cannot verify removal: ") + error.what());
        }
    }
    report.requireSuccess(request.instances.size());
    return report.rebootRequired;
}
void rescanDevices() {
    DEVINST root{};
    auto status = CM_Locate_DevNodeW(&root, nullptr, CM_LOCATE_DEVNODE_NORMAL);
    if (status == CR_SUCCESS)
        status = CM_Reenumerate_DevNode(root, CM_REENUMERATE_SYNCHRONOUS);
    if (status != CR_SUCCESS)
        fail("Device rescan failed: " + std::to_string(status));
}
}
