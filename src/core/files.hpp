#pragma once
#include <cstddef>
#include <filesystem>
#include <string>
namespace fpga {
std::string readFile(const std::filesystem::path& path, std::size_t limit);
}
