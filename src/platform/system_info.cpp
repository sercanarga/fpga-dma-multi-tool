#include "platform/devices.hpp"
#include "platform/registry.hpp"
#include "platform/service.hpp"
#include <wbemidl.h>
#include <wrl/client.h>
namespace fpga {
namespace {
using Microsoft::WRL::ComPtr;
class Bstr {
    BSTR value_;

  public:
    explicit Bstr(const wchar_t* text) : value_(SysAllocString(text)) {
        if (!value_)
            fail("Cannot allocate WMI string");
    }
    ~Bstr() {
        SysFreeString(value_);
    }
    Bstr(const Bstr&) = delete;
    Bstr& operator=(const Bstr&) = delete;
    operator BSTR() const {
        return value_;
    }
};
class Variant {
  public:
    VARIANT value{};
    Variant() {
        VariantInit(&value);
    }
    ~Variant() {
        VariantClear(&value);
    }
    Variant(const Variant&) = delete;
    Variant& operator=(const Variant&) = delete;
};
std::string property(IWbemClassObject* object, const wchar_t* name) {
    Variant value;
    if (!object || FAILED(object->Get(name, 0, &value.value, nullptr, nullptr)) ||
        value.value.vt == VT_NULL || value.value.vt == VT_EMPTY)
        return "Unavailable";
    if (value.value.vt == VT_BOOL)
        return value.value.boolVal == VARIANT_FALSE ? "No" : "Yes";
    Variant converted;
    if (FAILED(VariantChangeType(&converted.value, &value.value, 0, VT_BSTR)))
        return "Unavailable";
    return win::utf8(
        std::wstring_view(converted.value.bstrVal, SysStringLen(converted.value.bstrVal)));
}
std::string contains(IWbemClassObject* object, const wchar_t* name, LONG expected) {
    Variant value;
    if (!object || FAILED(object->Get(name, 0, &value.value, nullptr, nullptr)) ||
        value.value.vt != (VT_ARRAY | VT_I4))
        return "Unavailable";
    LONG first{}, last{};
    auto* array = value.value.parray;
    if (!array || SafeArrayGetDim(array) != 1 || FAILED(SafeArrayGetLBound(array, 1, &first)) ||
        FAILED(SafeArrayGetUBound(array, 1, &last)))
        return "Unavailable";
    for (LONG index = first; index <= last; ++index) {
        LONG item{};
        if (FAILED(SafeArrayGetElement(array, &index, &item)))
            return "Unavailable";
        if (item == expected)
            return "Yes";
    }
    return "No";
}
class Wmi {
    ComPtr<IWbemServices> service_;
    Context& context_;

  public:
    Wmi(const wchar_t* name, Context& context) : context_(context) {
        context.check();
        ComPtr<IWbemLocator> locator;
        auto status = CoCreateInstance(CLSID_WbemLocator, nullptr, CLSCTX_INPROC_SERVER,
                                       IID_PPV_ARGS(locator.GetAddressOf()));
        if (SUCCEEDED(status))
            status = locator->ConnectServer(Bstr(name), nullptr, nullptr, nullptr,
                                            WBEM_FLAG_CONNECT_USE_MAX_WAIT, nullptr, nullptr,
                                            service_.GetAddressOf());
        if (SUCCEEDED(status))
            status = CoSetProxyBlanket(service_.Get(), RPC_C_AUTHN_WINNT, RPC_C_AUTHZ_NONE, nullptr,
                                       RPC_C_AUTHN_LEVEL_CALL, RPC_C_IMP_LEVEL_IMPERSONATE, nullptr,
                                       EOAC_NONE);
        if (FAILED(status))
            fail("WMI unavailable: " + win::error(static_cast<DWORD>(status)));
    }
    ComPtr<IWbemClassObject> first(const wchar_t* query) {
        context_.check();
        ComPtr<IEnumWbemClassObject> rows;
        auto status = service_->ExecQuery(Bstr(L"WQL"), Bstr(query),
                                          WBEM_FLAG_FORWARD_ONLY | WBEM_FLAG_RETURN_IMMEDIATELY,
                                          nullptr, rows.GetAddressOf());
        if (FAILED(status))
            fail("WMI query failed: " + win::error(static_cast<DWORD>(status)));
        for (;;) {
            context_.check();
            ComPtr<IWbemClassObject> row;
            ULONG count{};
            status = rows->Next(250, 1, row.GetAddressOf(), &count);
            if (FAILED(status))
                fail("WMI enumeration failed: " + win::error(static_cast<DWORD>(status)));
            if (count || status == WBEM_S_FALSE)
                return row;
        }
    }
    std::string ready(IWbemClassObject* tpm) {
        Variant path;
        if (!tpm || FAILED(tpm->Get(L"__PATH", 0, &path.value, nullptr, nullptr)) ||
            path.value.vt != VT_BSTR)
            return "Unavailable";
        context_.check();
        ComPtr<IWbemCallResult> call;
        auto status =
            service_->ExecMethod(path.value.bstrVal, Bstr(L"IsReady"), WBEM_FLAG_RETURN_IMMEDIATELY,
                                 nullptr, nullptr, nullptr, call.GetAddressOf());
        if (FAILED(status))
            return "Unavailable";
        ComPtr<IWbemClassObject> result;
        do {
            context_.check();
            status = call->GetResultObject(250, result.ReleaseAndGetAddressOf());
        } while (status == WBEM_S_TIMEDOUT);
        if (FAILED(status) || property(result.Get(), L"ReturnValue") != "0")
            return "Unavailable";
        return property(result.Get(), L"IsReady");
    }
};
}
TableResult systemInformation(Context& context) {
    win::ComApartment apartment;
    TableResult result{{"Property", "Value"}, {}, "System information refreshed"};
    auto& rows = result.rows;
    const auto group = [&](const char* label, auto operation) {
        try {
            operation();
        } catch (const std::exception& error) {
            context.check();
            rows.push_back({label, std::string("Unavailable: ") + error.what()});
        }
    };
    group("System inventory", [&] {
        Wmi wmi(L"ROOT\\CIMV2", context);
        const auto add = [&](const wchar_t* query,
                             std::initializer_list<std::pair<const char*, const wchar_t*>> fields) {
            const auto object = wmi.first(query);
            for (const auto& [label, key] : fields)
                rows.push_back({label, property(object.Get(), key)});
        };
        add(L"SELECT Name,Manufacturer,VMMonitorModeExtensions,VirtualizationFirmwareEnabled FROM "
            L"Win32_Processor",
            {{"Processor", L"Name"},
             {"Processor manufacturer", L"Manufacturer"},
             {"Virtualization supported", L"VMMonitorModeExtensions"},
             {"Virtualization enabled in firmware", L"VirtualizationFirmwareEnabled"}});
        add(L"SELECT Manufacturer,Model,HypervisorPresent FROM Win32_ComputerSystem",
            {{"System manufacturer", L"Manufacturer"},
             {"System model", L"Model"},
             {"Hypervisor present", L"HypervisorPresent"}});
        add(L"SELECT Manufacturer,SMBIOSBIOSVersion FROM Win32_BIOS",
            {{"BIOS manufacturer", L"Manufacturer"}, {"BIOS version", L"SMBIOSBIOSVersion"}});
        add(L"SELECT Caption,Version,BuildNumber FROM Win32_OperatingSystem",
            {{"Windows", L"Caption"},
             {"Windows version", L"Version"},
             {"Windows build", L"BuildNumber"}});
    });
    group("Device Guard", [&] {
        Wmi wmi(L"ROOT\\Microsoft\\Windows\\DeviceGuard", context);
        const auto guard = wmi.first(L"SELECT * FROM Win32_DeviceGuard");
        rows.push_back({"Device Guard available", guard ? "Yes" : "No"});
        rows.push_back(
            {"DMA protection available", contains(guard.Get(), L"AvailableSecurityProperties", 3)});
        const auto vbs = property(guard.Get(), L"VirtualizationBasedSecurityStatus");
        rows.push_back({"VBS status", vbs == "0"   ? "Off"
                                      : vbs == "1" ? "Enabled, not running"
                                      : vbs == "2" ? "Running"
                                                   : "Unavailable"});
        for (const auto& [label, id] :
             {std::pair{"Credential Guard", 1L}, {"Memory integrity", 2L}, {"Secure launch", 3L}}) {
            rows.push_back({std::string(label) + " configured",
                            contains(guard.Get(), L"SecurityServicesConfigured", id)});
            rows.push_back({std::string(label) + " running",
                            contains(guard.Get(), L"SecurityServicesRunning", id)});
        }
    });
    group("TPM", [&] {
        Wmi wmi(L"ROOT\\CIMV2\\Security\\MicrosoftTpm", context);
        const auto tpm = wmi.first(L"SELECT * FROM Win32_Tpm");
        rows.push_back({"TPM present", tpm ? "Yes" : "No"});
        rows.push_back({"TPM ready", wmi.ready(tpm.Get())});
        rows.push_back({"TPM enabled", property(tpm.Get(), L"IsEnabled_InitialValue")});
        rows.push_back({"TPM activated", property(tpm.Get(), L"IsActivated_InitialValue")});
        rows.push_back({"TPM specification", property(tpm.Get(), L"SpecVersion")});
    });
    FIRMWARE_TYPE firmware{};
    rows.push_back({"Firmware", GetFirmwareType(&firmware)
                                    ? (firmware == FirmwareTypeUefi ? "UEFI" : "Legacy BIOS")
                                    : "Unavailable"});
    const auto secureBoot =
        win::RegistryKey(HKEY_LOCAL_MACHINE,
                         L"SYSTEM\\CurrentControlSet\\Control\\SecureBoot\\State")
            .number(L"UEFISecureBootEnabled");
    rows.push_back(
        {"Secure Boot enabled", secureBoot ? (*secureBoot ? "Yes" : "No") : "Unavailable"});
    for (const auto& device : windowsDevices(context, true)) {
        if (!device.currentWidth || !*device.currentWidth)
            continue;
        const auto prefix = "PCIe / " + device.name + " / " + device.instance;
        rows.push_back({prefix + " / Current width", "x" + std::to_string(*device.currentWidth)});
        rows.push_back(
            {prefix + " / Maximum width",
             device.maximumWidth ? "x" + std::to_string(*device.maximumWidth) : "Unavailable"});
    }
    return result;
}
}
