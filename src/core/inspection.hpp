#pragma once
#include <cstdint>
#include <map>
#include <string>
#include <vector>
namespace fpga {
using Row = std::vector<std::string>;
using Rows = std::vector<Row>;
std::vector<std::uint32_t> parseChain(const std::string& output);
std::uint64_t parseDna(const std::string& output);
std::uint32_t parseRegister(const std::string& output);
Rows configurationRows(const std::map<std::string, std::uint32_t>& registers);
Rows sensorRows(const std::string& output);
Rows flashRows(const std::string& output);
}
