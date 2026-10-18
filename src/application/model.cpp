#include "application/model.hpp"
#include "core/error.hpp"
#include "core/text.hpp"
#include <set>
#include <type_traits>
namespace fpga {
std::string pathText(const std::filesystem::path& path) {
    auto text = path.u8string();
    return {text.begin(), text.end()};
}
std::filesystem::path pathFromText(const std::string& text) {
    return std::filesystem::path(std::u8string(text.begin(), text.end()));
}
void Connection::validate() const {
    if (adapter < -1 || adapter > 15)
        fail("CH347 adapter must be -1 through 15");
    if (serial.size() > 256 || serial.find('\0') != serial.npos ||
        pathText(library).find('\0') != std::string::npos)
        fail("Invalid connection identifier or library path");
    if (backend != Backend::automatic && backend != Backend::ch347 && backend != Backend::ftdi)
        fail("Invalid writer backend");
}
void Request::validate() const {
    const auto* benchmark = std::get_if<BenchmarkRequest>(&operation);
    const bool continuous = benchmark && benchmark->settings.duration == 0 && timeout.count() == 0;
    if (!continuous && (timeout < std::chrono::seconds(1) || timeout > std::chrono::minutes(35)))
        fail("Operation timeout must be 1-2100 seconds");
    std::visit(
        [](const auto& request) {
            using T = std::decay_t<decltype(request)>;
            if constexpr (std::is_same_v<T, ScanRequest> || std::is_same_v<T, InspectRequest>) {
                request.connection.validate();
                if (request.index < (std::is_same_v<T, ScanRequest> ? -1 : 0) || request.index > 15)
                    fail("Invalid JTAG chain index");
                if constexpr (std::is_same_v<T, InspectRequest>)
                    if (request.kind != Inspection::registers &&
                        request.kind != Inspection::sensors && request.kind != Inspection::flash)
                        fail("Invalid inspection operation");
            } else if constexpr (std::is_same_v<T, ProgramRequest>) {
                request.connection.validate();
                request.firmware.validateSettings();
            } else if constexpr (std::is_same_v<T, BenchmarkRequest>)
                request.settings.validate();
            else if constexpr (std::is_same_v<T, InventoryRequest>) {
                if (request.kind != Inventory::system && request.kind != Inventory::history &&
                    request.kind != Inventory::drivers)
                    fail("Invalid inventory operation");
            } else if constexpr (std::is_same_v<T, DriverRequest>) {
                if (request.kind != Driver::ch347 && request.kind != Driver::d3xx &&
                    request.kind != Driver::writer)
                    fail("Invalid driver operation");
                if (request.serial.size() > 256 || request.serial.find('\0') != request.serial.npos)
                    fail("Invalid writer serial number");
            } else if constexpr (std::is_same_v<T, RemoveHistoryRequest>) {
                if (request.instances.empty() || request.instances.size() > 4096)
                    fail("Select between 1 and 4096 device instances");
                std::set<std::string> unique;
                std::size_t bytes = 0;
                for (const auto& instance : request.instances) {
                    bytes += instance.size();
                    if (instance.empty() || instance.size() > 4096 ||
                        instance.find('\0') != instance.npos ||
                        !unique.insert(lower(instance)).second)
                        fail("Invalid or duplicate device instance");
                }
                if (bytes > 48000)
                    fail("Too many device instances in one removal. Select a smaller group.");
            }
        },
        operation);
}
std::chrono::steady_clock::time_point Request::deadline(std::chrono::milliseconds grace) const {
    return timeout.count() == 0 ? std::chrono::steady_clock::time_point::max()
                                : std::chrono::steady_clock::now() + timeout + grace;
}
bool Request::privileged() const {
    return std::holds_alternative<DriverRequest>(operation) ||
           std::holds_alternative<RemoveHistoryRequest>(operation) ||
           std::holds_alternative<RescanRequest>(operation);
}
bool Request::programming() const {
    return std::holds_alternative<ProgramRequest>(operation);
}
bool Request::inventory() const {
    return std::holds_alternative<InventoryRequest>(operation) ||
           std::holds_alternative<ScanRequest>(operation);
}
std::string resultMessage(const Result& result) {
    return std::visit(
        [](const auto& value) -> std::string {
            using T = std::decay_t<decltype(value)>;
            if constexpr (std::is_same_v<T, ScanResult>)
                return "Found " + std::to_string(value.devices.size()) + " FPGA device(s)";
            else if constexpr (std::is_same_v<T, HistoryResult>)
                return "Device history refreshed";
            else if constexpr (std::is_same_v<T, BenchmarkResult>)
                return "Measurement completed";
            else if constexpr (std::is_same_v<T, ProgramResult>)
                return value.destination == Destination::flash ? "Flash written and verified"
                                                               : "SRAM bitstream loaded";
            else
                return value.message;
        },
        result);
}
}
