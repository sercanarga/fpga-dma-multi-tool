#pragma once
#include "core/benchmark.hpp"
#include "core/log.hpp"
#include <memory>
namespace fpga {
class LeechCore {
    class Implementation;
    std::unique_ptr<Implementation> implementation_;

  public:
    LeechCore(const Benchmark& settings, const Log& log);
    ~LeechCore();
    LeechCore(const LeechCore&) = delete;
    LeechCore& operator=(const LeechCore&) = delete;
    void restrict(std::uint64_t address, std::uint64_t length);
    bool tryRead(std::uint64_t address, std::span<std::uint8_t> data);
    void read(std::uint64_t address, std::span<std::uint8_t> data);
    void write(std::uint64_t address, std::span<std::uint8_t> data);
};
}
