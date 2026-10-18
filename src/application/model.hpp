#pragma once
#include "core/benchmark.hpp"
#include "core/device.hpp"
#include "core/firmware.hpp"
#include "core/inspection.hpp"
#include <chrono>
#include <optional>
#include <variant>
namespace fpga {
enum class Backend { automatic, ch347, ftdi };
struct Connection {
    Backend backend = Backend::automatic;
    int adapter = -1;
    std::string serial;
    std::filesystem::path library;
    void validate() const;
};
struct ScanRequest {
    Connection connection;
    int index = -1;
};
struct ProgramRequest {
    Connection connection;
    Firmware firmware;
    std::optional<std::uint64_t> expectedDna;
};
enum class Inspection { registers, sensors, flash };
struct InspectRequest {
    Connection connection;
    int index = 0;
    Inspection kind = Inspection::registers;
    std::optional<std::uint64_t> expectedDna;
};
struct BenchmarkRequest {
    Benchmark settings;
};
enum class Inventory { system, history, drivers };
struct InventoryRequest {
    Inventory kind;
};
enum class Driver { ch347, d3xx, writer };
struct DriverRequest {
    Driver kind;
    bool remove = false;
    std::string serial;
};
struct RemoveHistoryRequest {
    std::vector<std::string> instances;
};
struct RescanRequest {};
using Operation =
    std::variant<ScanRequest, ProgramRequest, InspectRequest, BenchmarkRequest, InventoryRequest,
                 DriverRequest, RemoveHistoryRequest, RescanRequest>;
struct Request {
    Operation operation;
    std::chrono::milliseconds timeout{45000};
    void validate() const;
    std::chrono::steady_clock::time_point deadline(std::chrono::milliseconds grace = {}) const;
    bool privileged() const;
    bool programming() const;
    bool inventory() const;
};
struct ScanResult {
    std::vector<Device> devices;
};
struct TableResult {
    Row columns;
    Rows rows;
    std::string message;
};
struct HistoryEntry {
    std::string instance, name, deviceClass, enumerator;
    bool present = false;
};
struct HistoryResult {
    std::vector<HistoryEntry> devices;
};
struct BenchmarkResult {
    std::vector<Measurement> measurements;
};
struct ProgramResult {
    Device device;
    std::string sha256;
    Destination destination;
};
struct Notice {
    std::string message;
};
using Result =
    std::variant<ScanResult, TableResult, HistoryResult, BenchmarkResult, ProgramResult, Notice>;
std::string resultMessage(const Result& result);
std::string pathText(const std::filesystem::path& path);
std::filesystem::path pathFromText(const std::string& text);
}
