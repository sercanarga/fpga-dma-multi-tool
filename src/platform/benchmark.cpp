#include "core/text.hpp"
#include "platform/leechcore.hpp"
#include "platform/service.hpp"
#include <algorithm>
namespace fpga {
namespace {
Measurement measure(Context& context, LeechCore& device, const Benchmark& settings,
                    TransferMode operation, int size) {
    std::vector<std::uint8_t> data(static_cast<std::size_t>(size));
    ReadAddresses addresses(settings, size, std::random_device{}());
    if (operation == TransferMode::write)
        device.read(settings.address, data);
    std::uint64_t operations = 0;
    const auto start = std::chrono::steady_clock::now();
    auto lastSample = start;
    std::uint64_t sampledOperations = 0;
    if (context.log)
        context.log(std::string(operation == TransferMode::read ? "Reading " : "Writing ") +
                    std::to_string(size) + " B per operation" +
                    (operation == TransferMode::read && settings.randomRead
                         ? " at random addresses\n"
                         : " at one address\n"));
    if (context.log && operation == TransferMode::read && size < 4096)
        context.log("Each sample reads a fresh 4 KiB page; throughput counts requested bytes.\n");
    do {
        context.check();
        if (operation == TransferMode::write)
            device.write(settings.address, data);
        else
            device.read(addresses.next(), data);
        ++operations;
        const auto now = std::chrono::steady_clock::now();
        const double interval = std::chrono::duration<double>(now - lastSample).count();
        if (interval >= 1.0) {
            const double rate = static_cast<double>(operations - sampledOperations) / interval;
            if (context.log)
                context.log(std::string(operation == TransferMode::read ? "Read: " : "Write: ") +
                            decimal(rate, 0) +
                            (operation == TransferMode::read ? " reads/s | " : " writes/s | ") +
                            decimal(rate * size / (1024.0 * 1024.0), 2) + " MiB/s | " +
                            decimal(1000000.0 / rate, 2) + " us\n");
            sampledOperations = operations;
            lastSample = now;
        }
    } while (settings.duration == 0 ||
             std::chrono::steady_clock::now() - start < std::chrono::seconds(settings.duration));
    const double seconds =
        std::chrono::duration<double>(std::chrono::steady_clock::now() - start).count();
    if (operation == TransferMode::write) {
        std::vector<std::uint8_t> verify(data.size());
        device.read(settings.address, verify);
        if (verify != data)
            fail("Target buffer changed during write test. Keep the dedicated buffer idle.");
    }
    const double rate = static_cast<double>(operations) / seconds;
    Measurement result{operation, size, rate * size / (1024.0 * 1024.0), rate, 1000000.0 / rate};
    if (context.log)
        context.log(std::string(operation == TransferMode::read ? "Read" : "Write") + " " +
                    std::to_string(size) + " B: " + decimal(result.mibPerSecond, 2) + " MiB/s\n");
    return result;
}
}
BenchmarkResult benchmark(const BenchmarkRequest& request, Context& context) {
    auto settings = request.settings;
    settings.validate();
    LeechCore device(settings, context.log);
    if (!settings.address) {
        if (context.log)
            context.log("Finding a readable DMA range...\n");
        const auto probeSize = settings.randomRead
                                   ? settings.length
                                   : static_cast<std::uint64_t>(*std::max_element(
                                         settings.sizes.begin(), settings.sizes.end()));
        std::vector<std::uint8_t> probe(static_cast<std::size_t>(probeSize));
        settings.address = findReadAddress([&](std::uint64_t address) {
            context.check();
            return device.tryRead(address, probe);
        });
        settings.length = (probe.size() + 4095ULL) & ~4095ULL;
        device.restrict(settings.address, settings.length);
    }
    if (context.log)
        context.log("Test range: 0x" + hex(settings.address, 16) + " - 0x" +
                    hex(settings.address + settings.length - 1, 16) + "\n");
    BenchmarkResult result;
    for (int size : settings.sizes) {
        if (settings.mode != TransferMode::write)
            result.measurements.push_back(
                measure(context, device, settings, TransferMode::read, size));
        if (settings.mode != TransferMode::read)
            result.measurements.push_back(
                measure(context, device, settings, TransferMode::write, size));
    }
    return result;
}
}
