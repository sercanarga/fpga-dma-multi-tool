#pragma once
#include <cstdint>
#include <filesystem>
#include <span>
#include <string>
#include <vector>
namespace fpga {
enum class Destination { sram, flash };
struct Firmware {
    std::filesystem::path path;
    Destination destination = Destination::sram;
    std::string part = "xc7a100t";
    int index = 0;
    void validateSettings() const;
    void validate() const;
};
std::vector<std::uint8_t> readFirmware(const std::filesystem::path& path);
void validateImage(std::span<const std::uint8_t> data, Destination destination,
                   const std::string& part);
bool firmwareExtension(const std::filesystem::path& path);
}
