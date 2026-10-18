#pragma once
#include <cstdint>
#include <string>
#include <string_view>
#include <vector>
namespace fpga {
std::string trim(std::string_view text);
std::string lower(std::string text);
std::vector<std::string> split(std::string_view text, char separator);
std::string join(const std::vector<std::string>& items, std::string_view separator);
std::string hex(std::uint64_t value, int width);
std::uint64_t number(std::string_view text, int base = 10);
std::string decimal(double value, int precision);
}
