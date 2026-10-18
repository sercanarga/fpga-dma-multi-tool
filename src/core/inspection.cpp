#include "core/inspection.hpp"
#include "core/device.hpp"
#include "core/error.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <cmath>
#include <regex>
namespace fpga {
std::vector<std::uint32_t> parseChain(const std::string& output) {
    const std::regex pattern(R"(index\s+(\d+)\s*:\s*\r?\n\s*idcode\s+(?:0x)?([0-9a-fA-F]{8})\b)");
    std::vector<std::uint32_t> chain;
    for (auto it = std::sregex_iterator(output.begin(), output.end(), pattern);
         it != std::sregex_iterator{}; ++it) {
        if (number((*it)[1].str()) != chain.size() || chain.size() >= 16)
            fail("Non-contiguous or oversized JTAG chain");
        auto id = static_cast<std::uint32_t>(number((*it)[2].str(), 16));
        partName(id);
        chain.push_back(id);
    }
    if (chain.empty())
        fail("Programmer returned no supported JTAG devices");
    return chain;
}
std::uint64_t parseDna(const std::string& output) {
    std::smatch match;
    if (!std::regex_search(output, match,
                           std::regex(R"re("dna"\s*:\s*"(?:0x)?([0-9a-fA-F]{1,16})")re")))
        fail("Programmer returned no DNA value");
    const auto value = number(match[1].str(), 16);
    if (!value || value == UINT64_MAX)
        fail("Invalid FPGA DNA");
    return value;
}
std::uint32_t parseRegister(const std::string& output) {
    std::smatch match;
    if (!std::regex_search(
            output, match,
            std::regex(R"(register raw value:\s*(?:0x)?([0-9a-f]{1,8})\b)", std::regex::icase)))
        fail("Register response could not be parsed");
    return static_cast<std::uint32_t>(number(match[1].str(), 16));
}
Rows configurationRows(const std::map<std::string, std::uint32_t>& registers) {
    Rows rows;
    const auto add = [&](std::string key, std::string value) {
        rows.push_back({std::move(key), std::move(value)});
    };
    for (const auto& [key, value] : registers)
        add(key + " raw", "0x" + hex(value, 8));
    if (registers.contains("STAT")) {
        auto v = registers.at("STAT");
        add("Configuration", (v & (1U << 14)) && (v & (1U << 4)) ? "Configured" : "Not complete");
        add("End of startup", v & (1U << 4) ? "Yes" : "No");
        add("INIT_B", v & (1U << 12) ? "High" : "Low");
        for (const auto& [bit, name] :
             std::vector<std::pair<int, std::string>>{{0, "CRC error"},
                                                      {15, "IDCODE error"},
                                                      {16, "Decryption error"},
                                                      {17, "XADC over-temperature"}})
            add(name, v & (1U << bit) ? "Detected" : "Clear");
        add("Configuration bus width",
            std::vector<std::string>{"x1", "x8", "x16", "x32"}[(v >> 25) & 3]);
    }
    if (registers.contains("CTRL0")) {
        auto v = registers.at("CTRL0");
        add("User I/O", v & 1 ? "Active" : "3-stated");
        add("PERSIST", v & 8 ? "Enabled" : "Disabled");
        add("Readback security",
            std::vector<std::string>{"Readback enabled", "Readback disabled",
                                     "Readback and reconfiguration disabled",
                                     "Readback and reconfiguration disabled"}[(v >> 4) & 3]);
        add("Configuration fallback", v & (1U << 10) ? "Enabled" : "Disabled");
        add("Over-temperature power-down", v & (1U << 12) ? "Enabled" : "Disabled");
    }
    if (registers.contains("BOOTSTS")) {
        const auto v = registers.at("BOOTSTS");
        const std::vector<std::string> states{"Fallback",     "IPROG",     "Watchdog timeout",
                                              "IDCODE error", "CRC error", "Address wrap error",
                                              "HMAC error"};
        for (int record = 0; record < 2; ++record) {
            const int offset = record * 8;
            std::vector<std::string> flags;
            if (!(v & (1U << offset)))
                flags.push_back("No valid boot record");
            else {
                for (int i = 0; i < 7; ++i)
                    if (v & (1U << (offset + i + 1)))
                        flags.push_back(states[static_cast<std::size_t>(i)]);
                if (flags.empty())
                    flags.push_back("Normal configuration");
            }
            add("Boot status " + std::to_string(record), join(flags, ", "));
        }
    }
    if (registers.contains("WBSTAR")) {
        auto v = registers.at("WBSTAR");
        add("Warm boot start address", "0x" + hex(v & 0x1fffffffU, 8));
        add("RS[1:0]", std::string{static_cast<char>('0' + ((v >> 31) & 1)),
                                   static_cast<char>('0' + ((v >> 30) & 1))});
        add("RS pins", v & (1U << 29) ? "Enabled" : "3-stated");
    }
    return rows;
}
Rows sensorRows(const std::string& output) {
    const auto measurement = [&](const std::string& key) {
        std::smatch match;
        if (!std::regex_search(
                output, match,
                std::regex('"' + key + R"("\s*:\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?))")))
            fail("Missing XADC measurement: " + key);
        auto v = std::stod(match[1].str());
        if (!std::isfinite(v))
            fail("Invalid XADC measurement");
        return v;
    };
    Rows rows;
    for (const auto& key : {std::string("temp"), std::string("vccint"), std::string("vccaux")}) {
        const int precision = key == "temp" ? 2 : 3;
        rows.push_back({key, decimal(measurement(key), precision) + (key == "temp" ? " C" : " V") +
                                 " (min " + decimal(measurement("min" + key), precision) +
                                 ", max " + decimal(measurement("max" + key), precision) + ")"});
    }
    std::smatch raw, bram;
    if (std::regex_search(output, raw, std::regex(R"("raw"\s*:\s*\{([^}]+)\})"))) {
        const auto text = raw[1].str();
        if (std::regex_search(text, bram, std::regex(R"("6"\s*:\s*(\d+))"))) {
            auto v = number(bram[1].str());
            if (v > 65535)
                fail("Invalid XADC VCCBRAM register");
            rows.push_back(
                {"VCCBRAM", decimal(static_cast<double>(v >> 4) / 4096.0 * 3.0, 3) + " V"});
        }
    }
    return rows;
}
Rows flashRows(const std::string& output) {
    Rows rows;
    bool identified = false;
    for (const auto& line : split(output, '\n')) {
        const auto text = trim(line);
        std::smatch match;
        if (std::regex_match(
                text, match,
                std::regex(R"(Detected:\s+(\S+)\s+(.+?)\s+\d+\s+sectors\s+size:\s+(\d+)Mb\s*)",
                           std::regex::icase))) {
            rows.push_back({"Manufacturer", match[1].str()});
            rows.push_back({"Model", match[2].str()});
            rows.push_back({"Capacity", match[3].str() + " Mbit"});
            continue;
        }
        const auto separator = text.find(':');
        if (separator == text.npos)
            continue;
        const auto key = trim(text.substr(0, separator)), value = trim(text.substr(separator + 1)),
                   label = lower(key);
        const std::vector<std::string> keys{"jedec",    "manufacturer", "model",
                                            "capacity", "status",       "memory type"};
        if (label == "rdsr" || label == "bp" || label == "tb" || label == "qe" ||
            std::any_of(keys.begin(), keys.end(),
                        [&](const auto& k) { return label.find(k) != label.npos; })) {
            rows.push_back({key, value});
            if (label.find("jedec") != label.npos &&
                std::regex_search(value, std::regex(R"((?:0x)?[0-9a-fA-F]{2,8})")))
                identified = true;
        }
    }
    if (!identified)
        fail("Programmer returned no SPI flash JEDEC identity");
    return rows;
}
}
