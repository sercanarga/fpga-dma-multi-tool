#include "core/firmware.hpp"
#include "core/device.hpp"
#include "core/error.hpp"
#include "core/files.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <array>
namespace fpga {
namespace {
bool synchronization(std::span<const std::uint8_t> data, bool reversed = false) {
    const std::array<std::uint8_t, 4> word =
        reversed ? std::array<std::uint8_t, 4>{0x55, 0x99, 0xaa, 0x66}
                 : std::array<std::uint8_t, 4>{0xaa, 0x99, 0x55, 0x66};
    return std::search(data.begin(), data.end(), word.begin(), word.end()) != data.end();
}
class Reader {
    std::span<const std::uint8_t> data_;

  public:
    explicit Reader(std::span<const std::uint8_t> data) : data_(data) {}
    std::span<const std::uint8_t> take(std::size_t size) {
        if (size > data_.size())
            fail("Truncated bitstream");
        auto result = data_.first(size);
        data_ = data_.subspan(size);
        return result;
    }
    std::uint32_t integer(std::size_t size) {
        std::uint32_t value = 0;
        for (auto byte : take(size))
            value = (value << 8) | byte;
        return value;
    }
    std::size_t remaining() const {
        return data_.size();
    }
};
}
bool firmwareExtension(const std::filesystem::path& path) {
    const auto extension = lower(path.extension().string());
    return extension == ".bit" || extension == ".bin";
}
std::vector<std::uint8_t> readFirmware(const std::filesystem::path& path) {
    const auto data = readFile(path, 32 * 1024 * 1024);
    if (data.empty())
        fail("Firmware must not be empty");
    return {data.begin(), data.end()};
}

void Firmware::validateSettings() const {
    if (path.native().find(std::filesystem::path::value_type{}) != path.native().npos)
        fail("Invalid firmware path");
    if (destination != Destination::sram && destination != Destination::flash)
        fail("Invalid firmware destination");
    if (index < 0 || index > 15 ||
        !std::any_of(catalog().begin(), catalog().end(),
                     [&](const Part& p) { return part == p.name; }))
        fail("Select a supported FPGA model and chain index");
    if (lower(path.extension().string()) != (destination == Destination::sram ? ".bit" : ".bin"))
        fail("SRAM requires .bit; flash requires .bin");
}
void Firmware::validate() const {
    validateSettings();
    validateImage(readFirmware(path), destination, part);
}
void validateImage(std::span<const std::uint8_t> data, Destination destination,
                   const std::string& part) {
    if (data.empty() || data.size() > 32 * 1024 * 1024)
        fail("Invalid firmware size");
    if (destination == Destination::flash) {
        if (!synchronization(data) && !synchronization(data, true))
            fail("No Artix-7 synchronization word in firmware");
        return;
    }
    Reader reader(data);
    constexpr std::array<std::uint8_t, 9> header{0x0f, 0xf0, 0x0f, 0xf0, 0x0f, 0xf0, 0x0f, 0xf0, 0};
    if (reader.integer(2) != header.size())
        fail("Invalid Xilinx bitstream header");
    const auto signature = reader.take(header.size());
    if (!std::equal(signature.begin(), signature.end(), header.begin()) || reader.integer(2) != 1)
        fail("Invalid bitstream field marker");
    std::string filePart;
    for (const char key : {'a', 'b', 'c', 'd'}) {
        if (reader.integer(1) != static_cast<unsigned char>(key))
            fail("Invalid bitstream metadata");
        const auto size = reader.integer(2);
        if (!size || size > 4096)
            fail("Invalid bitstream metadata length");
        const auto field = reader.take(size);
        if (key == 'b') {
            filePart.assign(field.begin(), field.end());
            std::erase(filePart, '\0');
            filePart = lower(filePart);
        }
    }
    auto normalized = filePart.starts_with("xc") ? filePart.substr(2) : filePart;
    auto expected = part.starts_with("xc") ? part.substr(2) : part;
    if (!normalized.starts_with(expected) ||
        (normalized.size() > expected.size() && normalized[expected.size()] >= '0' &&
         normalized[expected.size()] <= '9'))
        fail("Bitstream targets " + filePart + ", selected " + part);
    if (reader.integer(1) != 'e')
        fail("Missing bitstream payload");
    const auto length = reader.integer(4);
    if (!length || length != reader.remaining())
        fail("Invalid bitstream payload length");
    if (!synchronization(reader.take(length)))
        fail("No Artix-7 synchronization word in bitstream");
}
}
