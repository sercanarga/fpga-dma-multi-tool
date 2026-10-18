#include "core/jtag.hpp"
#include "core/error.hpp"

namespace fpga {
Bits bits(std::uint64_t value, int count) {
    if (count < 0 || count > 64)
        fail("Invalid bit count");
    Bits result(static_cast<std::size_t>(count));
    for (int i = 0; i < count; ++i)
        result[static_cast<std::size_t>(i)] = (value >> i) & 1;
    return result;
}
std::uint64_t readBits(const Bits& input, int offset, int count) {
    if (offset < 0 || count < 0 || count > 64 ||
        static_cast<std::size_t>(offset) + count > input.size())
        fail("Truncated JTAG response");
    std::uint64_t value = 0;
    for (int i = 0; i < count; ++i)
        value |= std::uint64_t(input[static_cast<std::size_t>(offset + i)] != 0) << i;
    return value;
}
Bytes pack(const Bits& input) {
    Bytes result(static_cast<std::size_t>((input.size() + 7) / 8), '\0');
    for (std::size_t i = 0; i < input.size(); ++i)
        if (input[i])
            result[static_cast<std::size_t>(i / 8)] |= static_cast<char>(1U << (i % 8));
    return result;
}
Bits unpack(std::span<const std::uint8_t> input, int count) {
    if (count < 0 || (static_cast<std::size_t>(count) + 7) / 8 > input.size())
        fail("Truncated packed JTAG response");
    Bits result(static_cast<std::size_t>(count));
    for (int i = 0; i < count; ++i)
        result[static_cast<std::size_t>(i)] =
            (static_cast<unsigned char>(input[i / 8]) >> (i % 8)) & 1;
    return result;
}
Bytes ch347Command(const Bits& tms, const Bits& tdi) {
    if (tms.empty() || tms.size() > 248 || tms.size() != tdi.size())
        fail("Invalid CH347 transfer length");
    const auto length = tms.size() * 2 + 1;
    Bytes out;
    out.reserve(static_cast<std::size_t>(length + 3));
    out.push_back(static_cast<std::uint8_t>(0xd2));
    out.push_back(static_cast<std::uint8_t>(length & 0xff));
    out.push_back(static_cast<std::uint8_t>(length >> 8));
    std::uint8_t state = 0x20;
    for (std::size_t i = 0; i < tms.size(); ++i) {
        state = static_cast<std::uint8_t>(0x20 | (tms[i] ? 2 : 0) | (tdi[i] ? 0x10 : 0));
        out.push_back(state);
        out.push_back(static_cast<std::uint8_t>(state | 1));
    }
    out.push_back(state);
    return out;
}
Bits ch347Response(std::span<const std::uint8_t> bytes, int count) {
    if (count <= 0 || count > 248 || bytes.size() != static_cast<std::size_t>(count + 3) ||
        static_cast<unsigned char>(bytes[0]) != 0xd2 ||
        (static_cast<unsigned char>(bytes[1]) | (static_cast<unsigned char>(bytes[2]) << 8)) !=
            count)
        fail("Invalid CH347 response");
    Bits out(static_cast<std::size_t>(count));
    for (int i = 0; i < count; ++i)
        out[static_cast<std::size_t>(i)] = bytes[i + 3] & 1;
    return out;
}
void Jtag::reset() {
    transport_.shift({1, 1, 1, 1, 1, 1, 0}, Bits(7));
}
Bits Jtag::scanRegister(bool instruction, const Bits& input) {
    if (input.empty())
        fail("Cannot scan an empty register");
    Bits tms = instruction ? Bits{1, 1, 0, 0} : Bits{1, 0, 0};
    const auto prefix = tms.size();
    Bits tdi(prefix);
    for (std::size_t i = 0; i < input.size(); ++i) {
        tms.push_back(i + 1 == input.size());
        tdi.push_back(input[i]);
    }
    tms.insert(tms.end(), {1, 0});
    tdi.insert(tdi.end(), {0, 0});
    auto out = transport_.shift(tms, tdi);
    if (out.size() != tms.size())
        fail("Truncated JTAG register");
    return Bits(out.begin() + static_cast<std::ptrdiff_t>(prefix), out.end() - 2);
}
std::vector<std::uint32_t> Jtag::detect() {
    reset();
    auto data = scanRegister(false, Bits(32 * 17));
    std::vector<std::uint32_t> chain;
    for (int i = 0; i < 17; ++i) {
        const auto id = static_cast<std::uint32_t>(readBits(data, i * 32, 32));
        if (id == 0 || id == 0xffffffffU)
            break;
        if (i == 16)
            fail("JTAG chain exceeds 16 devices");
        if (!(id & 1))
            fail("JTAG chain is not an IDCODE-only chain");
        partName(id);
        chain.push_back(id);
    }
    if (chain.empty())
        fail("No JTAG IDCODE. Check board power, cable and driver.");
    return chain;
}
std::uint64_t Jtag::readDna(int count, int target) {
    if (count < 1 || count > 16 || target < 0 || target >= count)
        fail("Invalid JTAG chain index");
    reset();
    Bits ir;
    for (int i = 0; i < count; ++i) {
        const auto value = bits(i == target ? 0x32 : 0x3f, 6);
        ir.insert(ir.end(), value.begin(), value.end());
    }
    scanRegister(true, ir);
    return readBits(scanRegister(false, Bits(static_cast<std::size_t>(64 + count - 1))), target,
                    64);
}
std::vector<Device> Jtag::scan(int selected, const std::string& target,
                               const std::string& backend) {
    const auto chain = detect();
    if (selected < -1 || selected >= static_cast<int>(chain.size()))
        fail("Selected device is outside the JTAG chain");
    std::vector<Device> result;
    for (int i = 0; i < static_cast<int>(chain.size()); ++i) {
        if (selected >= 0 && selected != i)
            continue;
        const auto first = readDna(static_cast<int>(chain.size()), i);
        if (first != readDna(static_cast<int>(chain.size()), i))
            fail("Unstable FUSE_DNA. Check cable and power.");
        if (!first || first == UINT64_MAX)
            fail("Invalid FUSE_DNA");
        const auto id = chain[static_cast<std::size_t>(i)];
        const auto part = partName(id);
        result.push_back(
            {i, part, target, backend, id, first, deviceDna(first), boardMatches(part, chain)});
    }
    reset();
    return result;
}
}
