#pragma once
#include <cstdint>
#include <string>
#include <vector>
namespace fpga {
struct Part {
    std::uint32_t id;
    const char* name;
};
const std::vector<Part>& catalog();
std::string partName(std::uint32_t id);
std::vector<std::string> boardMatches(const std::string& part,
                                      const std::vector<std::uint32_t>& chain);
std::uint64_t deviceDna(std::uint64_t fuse);
struct Device {
    int index = 0;
    std::string part, target, backend;
    std::uint32_t id = 0;
    std::uint64_t fuse = 0, dna = 0;
    std::vector<std::string> boards;
};
}
