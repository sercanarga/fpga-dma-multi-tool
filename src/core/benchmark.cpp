#include "core/benchmark.hpp"
#include "core/error.hpp"
#include <algorithm>
#include <array>
#include <limits>
namespace fpga {
bool readPageAligned(std::uint64_t address, std::span<std::uint8_t> data,
                     const std::function<bool(std::uint64_t, std::span<std::uint8_t>)>& read) {
    constexpr std::size_t pageSize = 4096;
    if (address > std::numeric_limits<std::uint64_t>::max() - data.size())
        return false;
    std::array<std::uint8_t, pageSize> page;
    while (!data.empty()) {
        const auto offset = static_cast<std::size_t>(address % pageSize);
        std::size_t count;
        if (offset || data.size() < pageSize) {
            if (!read(address - offset, page))
                return false;
            count = std::min(pageSize - offset, data.size());
            std::copy_n(page.begin() + offset, count, data.begin());
        } else {
            count = data.size() - data.size() % pageSize;
            if (!read(address, data.first(count)))
                return false;
        }
        address += count;
        data = data.subspan(count);
    }
    return true;
}
ReadAddresses::ReadAddresses(const Benchmark& settings, int bytes, std::uint64_t seed)
    : base_(settings.address), random_(settings.randomRead), generator_(seed) {
    settings.validate();
    if (!base_ || bytes < 4 || static_cast<std::uint64_t>(bytes) > settings.length)
        fail("Resolve a readable address range before testing");
    offsets_ = std::uniform_int_distribution<std::uint64_t>(0, (settings.length - bytes) / 8);
}
std::uint64_t ReadAddresses::next() {
    return base_ + (random_ ? offsets_(generator_) * 8 : 0);
}
std::uint64_t findReadAddress(const std::function<bool(std::uint64_t)>& readable) {
    for (auto address = automaticReadStride; address < automaticReadLimit;
         address += automaticReadStride)
        if (readable(address))
            return address;
    fail("No readable DMA address found. Check the target connection or set an address in "
         "Advanced.");
}
void Benchmark::validate() const {
    if (mode != TransferMode::read && mode != TransferMode::write && mode != TransferMode::both)
        fail("Invalid transfer mode");
    if (duration < 0 || duration > 60 || sizes.empty() || sizes.size() > 16)
        fail("Duration must be 0-60 seconds with 1-16 transfer sizes");
    if (duration == 0 && (mode != TransferMode::read || sizes.size() != 1))
        fail("Continuous testing requires read mode and one transfer size");
    if (mode != TransferMode::read && (!dedicated || !address))
        fail("Write tests require an explicit, dedicated target buffer");
    if (!address && length > automaticReadStride)
        fail("Automatic address selection supports buffers up to 16 MiB");
    if (address % 4096 || length < 4096 || length % 4096 || length > 256 * 1024 * 1024 ||
        address > std::numeric_limits<std::uint64_t>::max() - length)
        fail("Use a page-aligned target buffer up to 256 MiB");
    for (const int size : sizes)
        if (size < 4 || size > 16 * 1024 * 1024 || size % 4 ||
            static_cast<std::uint64_t>(size) > length)
            fail(
                "Transfer sizes must be aligned to 4 bytes and fit in the buffer (maximum 16 MiB)");
}
}
