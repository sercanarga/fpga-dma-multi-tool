#pragma once
#include "core/device.hpp"
#include <span>
namespace fpga {
using Bits = std::vector<std::uint8_t>;
using Bytes = std::vector<std::uint8_t>;
Bits bits(std::uint64_t value, int count);
std::uint64_t readBits(const Bits& input, int offset, int count);
Bytes pack(const Bits& input);
Bits unpack(std::span<const std::uint8_t> input, int count);
Bytes ch347Command(const Bits& tms, const Bits& tdi);
Bits ch347Response(std::span<const std::uint8_t> input, int count);
class Transport {
  public:
    virtual ~Transport() = default;
    virtual Bits shift(const Bits& tms, const Bits& tdi) = 0;
};
class Jtag {
  public:
    explicit Jtag(Transport& transport) : transport_(transport) {}
    std::vector<std::uint32_t> detect();
    std::uint64_t readDna(int count, int target);
    std::vector<Device> scan(int selected, const std::string& target, const std::string& backend);

  private:
    void reset();
    Bits scanRegister(bool instruction, const Bits& input);
    Transport& transport_;
};
}
