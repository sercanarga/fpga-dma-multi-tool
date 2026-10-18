#include "application/protocol.hpp"
#include "application/limits.hpp"
#include "core/error.hpp"
#include "core/text.hpp"
#include <nlohmann/json.hpp>
#include <type_traits>
namespace fpga {
namespace {
using Json = nlohmann::json;
template <class E> E enumeration(const Json& j, const char* key, int maximum, int fallback = 0) {
    const int value = j.value(key, fallback);
    if (value < 0 || value > maximum)
        fail(std::string("Invalid ") + key);
    return static_cast<E>(value);
}
Json connection(const Connection& c) {
    return {{"backend", c.backend},
            {"adapter", c.adapter},
            {"serial", c.serial},
            {"library", pathText(c.library)}};
}
Connection connection(const Json& j) {
    return {enumeration<Backend>(j, "backend", 2), j.value("adapter", -1),
            j.value("serial", std::string{}), pathFromText(j.value("library", std::string{}))};
}
Json device(const Device& d) {
    return {{"index", d.index}, {"part", d.part}, {"target", d.target}, {"backend", d.backend},
            {"id", d.id},       {"fuse", d.fuse}, {"dna", d.dna},       {"boards", d.boards}};
}
Device device(const Json& j) {
    return {j.at("index").get<int>(),
            j.at("part"),
            j.at("target"),
            j.at("backend"),
            j.at("id"),
            j.at("fuse"),
            j.at("dna"),
            j.at("boards").get<std::vector<std::string>>()};
}
Json parse(std::string_view text, std::size_t maximum) {
    if (text.empty() || text.size() > maximum)
        fail("Protocol payload exceeds bounds");
    auto j = Json::parse(text);
    if (!j.is_object())
        fail("Expected protocol object");
    return j;
}
std::optional<std::uint64_t> expected(const Json& j) {
    return j.contains("expected_dna") ? std::optional{j.at("expected_dna").get<std::uint64_t>()}
                                      : std::nullopt;
}
}
std::string encodeRequest(const Request& request) {
    request.validate();
    Json j{{"timeout_ms", request.timeout.count()}};
    std::visit(
        [&](const auto& value) {
            using T = std::decay_t<decltype(value)>;
            if constexpr (std::is_same_v<T, ScanRequest> || std::is_same_v<T, ProgramRequest> ||
                          std::is_same_v<T, InspectRequest>)
                j["connection"] = connection(value.connection);
            if constexpr (std::is_same_v<T, ProgramRequest> || std::is_same_v<T, InspectRequest>)
                if (value.expectedDna)
                    j["expected_dna"] = *value.expectedDna;
            if constexpr (std::is_same_v<T, ScanRequest>) {
                j["action"] = "scan";
                j["index"] = value.index;
            } else if constexpr (std::is_same_v<T, ProgramRequest>) {
                j["action"] = "program";
                j["path"] = pathText(value.firmware.path);
                j["part"] = value.firmware.part;
                j["destination"] = value.firmware.destination;
                j["index"] = value.firmware.index;
            } else if constexpr (std::is_same_v<T, InspectRequest>) {
                j["action"] = "inspect";
                j["kind"] = value.kind;
                j["index"] = value.index;
            } else if constexpr (std::is_same_v<T, BenchmarkRequest>) {
                const auto& b = value.settings;
                j["action"] = "benchmark";
                j["mode"] = b.mode;
                j["duration"] = b.duration;
                j["sizes"] = b.sizes;
                j["address"] = b.address;
                j["length"] = b.length;
                j["dedicated"] = b.dedicated;
                j["randomRead"] = b.randomRead;
            } else if constexpr (std::is_same_v<T, InventoryRequest>) {
                j["action"] = "inventory";
                j["kind"] = value.kind;
            } else if constexpr (std::is_same_v<T, DriverRequest>) {
                j["action"] = "driver";
                j["kind"] = value.kind;
                j["remove"] = value.remove;
                j["serial"] = value.serial;
            } else if constexpr (std::is_same_v<T, RemoveHistoryRequest>) {
                j["action"] = "remove-history";
                j["instances"] = value.instances;
            } else
                j["action"] = "rescan";
        },
        request.operation);
    auto encoded = j.dump(-1, ' ', false, Json::error_handler_t::replace);
    if (encoded.size() > limits::requestBytes)
        fail("Worker request exceeds 64 KiB");
    return encoded;
}
Request decodeRequest(std::string_view text) {
    auto j = parse(text, limits::requestBytes);
    Request request;
    request.timeout = std::chrono::milliseconds(j.value("timeout_ms", 45000));
    const auto action = j.at("action").get<std::string>();
    if (action == "scan")
        request.operation = ScanRequest{connection(j.at("connection")), j.value("index", -1)};
    else if (action == "program")
        request.operation = ProgramRequest{connection(j.at("connection")),
                                           {pathFromText(j.at("path")),
                                            enumeration<Destination>(j, "destination", 1),
                                            j.at("part"), j.value("index", 0)},
                                           expected(j)};
    else if (action == "inspect")
        request.operation = InspectRequest{connection(j.at("connection")), j.value("index", 0),
                                           enumeration<Inspection>(j, "kind", 2), expected(j)};
    else if (action == "benchmark")
        request.operation = BenchmarkRequest{
            {enumeration<TransferMode>(j, "mode", 2), j.value("duration", 0),
             j.at("sizes").get<std::vector<int>>(), j.at("address"), j.at("length"),
             j.value("dedicated", false), j.value("randomRead", true)}};
    else if (action == "inventory")
        request.operation = InventoryRequest{enumeration<Inventory>(j, "kind", 2)};
    else if (action == "driver")
        request.operation =
            DriverRequest{enumeration<Driver>(j, "kind", 2), j.value("remove", false),
                          j.value("serial", std::string{})};
    else if (action == "remove-history")
        request.operation = RemoveHistoryRequest{
            j.contains("instances")
                ? j.at("instances").get<std::vector<std::string>>()
                : std::vector<std::string>{j.at("instance").get<std::string>()}};
    else if (action == "rescan")
        request.operation = RescanRequest{};
    else
        fail("Unknown operation");
    request.validate();
    return request;
}
std::string encodeResult(const Result& result) {
    Json j;
    std::visit(
        [&](const auto& value) {
            using T = std::decay_t<decltype(value)>;
            if constexpr (std::is_same_v<T, ScanResult>) {
                j = {{"kind", "scan"}, {"devices", Json::array()}};
                for (const auto& d : value.devices)
                    j["devices"].push_back(device(d));
            } else if constexpr (std::is_same_v<T, TableResult>)
                j = {{"kind", "table"},
                     {"columns", value.columns},
                     {"rows", value.rows},
                     {"message", value.message}};
            else if constexpr (std::is_same_v<T, HistoryResult>) {
                j = {{"kind", "history"}, {"devices", Json::array()}};
                for (const auto& d : value.devices)
                    j["devices"].push_back({{"instance", d.instance},
                                            {"name", d.name},
                                            {"class", d.deviceClass},
                                            {"enumerator", d.enumerator},
                                            {"present", d.present}});
            } else if constexpr (std::is_same_v<T, BenchmarkResult>) {
                j = {{"kind", "benchmark"}, {"measurements", Json::array()}};
                for (const auto& m : value.measurements)
                    j["measurements"].push_back({{"operation", m.operation},
                                                 {"bytes", m.bytes},
                                                 {"mib_s", m.mibPerSecond},
                                                 {"ops_s", m.operationsPerSecond},
                                                 {"latency_us", m.latencyUs}});
            } else if constexpr (std::is_same_v<T, ProgramResult>)
                j = {{"kind", "program"},
                     {"device", device(value.device)},
                     {"sha256", value.sha256},
                     {"destination", value.destination}};
            else
                j = {{"kind", "notice"}, {"message", value.message}};
        },
        result);
    return j.dump(-1, ' ', false, Json::error_handler_t::replace);
}
Result decodeResult(std::string_view text) {
    auto j = parse(text, limits::eventBytes);
    const std::string kind = j.at("kind");
    if (kind == "scan") {
        ScanResult r;
        for (const auto& d : j.at("devices"))
            r.devices.push_back(device(d));
        return r;
    }
    if (kind == "table")
        return TableResult{j.at("columns").get<Row>(), j.at("rows").get<Rows>(), j.at("message")};
    if (kind == "history") {
        HistoryResult r;
        for (const auto& d : j.at("devices"))
            r.devices.push_back({d.at("instance"), d.at("name"), d.at("class"), d.at("enumerator"),
                                 d.at("present")});
        return r;
    }
    if (kind == "benchmark") {
        BenchmarkResult r;
        for (const auto& m : j.at("measurements"))
            r.measurements.push_back({enumeration<TransferMode>(m, "operation", 1), m.at("bytes"),
                                      m.at("mib_s"), m.at("ops_s"), m.at("latency_us")});
        return r;
    }
    if (kind == "program")
        return ProgramResult{device(j.at("device")), j.at("sha256"),
                             enumeration<Destination>(j, "destination", 1)};
    if (kind == "notice")
        return Notice{j.at("message")};
    fail("Unknown result");
}
std::string encodeEvent(const Event& event) {
    if (event.result)
        return Json{{"type", "result"}, {"data", Json::parse(encodeResult(*event.result))}}.dump(
            -1, ' ', false, Json::error_handler_t::replace);
    if (!event.error.empty())
        return Json{{"type", "error"}, {"message", event.error}}.dump(
            -1, ' ', false, Json::error_handler_t::replace);
    return Json{{"type", "log"}, {"text", event.log}}.dump(-1, ' ', false,
                                                           Json::error_handler_t::replace);
}
Event decodeEvent(std::string_view text) {
    auto j = parse(text, limits::eventBytes);
    const std::string type = j.at("type");
    if (type == "log")
        return {j.at("text"), {}, std::nullopt};
    if (type == "error") {
        auto error = j.at("message").get<std::string>();
        if (error.empty())
            fail("Worker error message is empty");
        return {{}, std::move(error), std::nullopt};
    }
    if (type == "result")
        return {{}, {}, decodeResult(j.at("data").dump())};
    fail("Unknown worker event");
}
}
