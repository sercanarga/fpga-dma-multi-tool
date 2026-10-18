#include "core/device.hpp"
#include "core/error.hpp"
#include "core/text.hpp"
#include <algorithm>

namespace fpga {
const std::vector<Part>& catalog() {
    static const std::vector<Part> parts{{0x0362e093, "xc7a15t"},  {0x0362d093, "xc7a35t"},
                                         {0x0362c093, "xc7a50t"},  {0x03632093, "xc7a75t"},
                                         {0x03631093, "xc7a100t"}, {0x03636093, "xc7a200t"}};
    return parts;
}
std::string partName(std::uint32_t id) {
    for (const auto& part : catalog())
        if ((id & 0x0fffffffU) == part.id)
            return part.name;
    fail("Unsupported Artix-7 IDCODE: " + hex(id, 8));
}
std::uint64_t deviceDna(std::uint64_t fuse) {
    std::uint64_t value = 0;
    for (int i = 0; i < 64; ++i) {
        value = (value << 1) | (fuse & 1);
        fuse >>= 1;
    }
    return value & ((std::uint64_t{1} << 57) - 1);
}
std::vector<std::string> boardMatches(const std::string& part,
                                      const std::vector<std::uint32_t>& chain) {
    std::vector<std::string> result;
    if (part == "xc7a35t")
        result = {"PCIeSquirrel",     "ScreamerM2",     "pciescreamer", "CaptainDMA M2 x1",
                  "CaptainDMA M2 x4", "CaptainDMA 35T", "GBOX",         "NeTV2 35T"};
    if (part == "xc7a75t")
        result = {"EnigmaX1", "CaptainDMA 75T"};
    if (part == "xc7a100t")
        result = {"CaptainDMA 100T", "ZDMA", "NeTV2 100T", "Litefury"};
    if (part == "xc7a200t")
        result = {"AC701 FT601", "Acorn"};
    if (chain.size() == 2 &&
        ((chain[0] & 0x0fffffffU) == 0x03632093 || (chain[1] & 0x0fffffffU) == 0x03632093) &&
        ((chain[0] & 0x0fffffffU) == 0x03631093 || (chain[1] & 0x0fffffffU) == 0x03631093) &&
        std::find(result.begin(), result.end(), "ZDMA") == result.end())
        result.push_back("ZDMA");
    return result;
}
}
