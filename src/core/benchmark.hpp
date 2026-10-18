#pragma once
#include <cstdint>
#include <functional>
#include <random>
#include <span>
#include <vector>
namespace fpga {
inline constexpr std::uint64_t automaticReadStride = 16 * 1024 * 1024;
inline constexpr std::uint64_t automaticReadLimit = 17 * automaticReadStride;
std::uint64_t findReadAddress(const std::function<bool(std::uint64_t)>& readable);
bool readPageAligned(std::uint64_t address, std::span<std::uint8_t> data,
                     const std::function<bool(std::uint64_t, std::span<std::uint8_t>)>& read);
enum class TransferMode { read, write, both };
struct Benchmark {
    TransferMode mode = TransferMode::read;
    int duration = 0;
    std::vector<int> sizes{8};
    std::uint64_t address = 0, length = 16 * 1024 * 1024;
    bool dedicated = false;
    bool randomRead = true;
    void validate() const;
};
class ReadAddresses {
    std::uint64_t base_;
    bool random_;
    std::mt19937_64 generator_;
    std::uniform_int_distribution<std::uint64_t> offsets_;

  public:
    ReadAddresses(const Benchmark& settings, int bytes, std::uint64_t seed);
    std::uint64_t next();
};
struct Measurement {
    TransferMode operation;
    int bytes;
    double mibPerSecond, operationsPerSecond, latencyUs;
};
}
