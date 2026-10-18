#include "application/worker.hpp"
#include "application/limits.hpp"
#include "application/protocol.hpp"
#include "platform/service.hpp"
#include <iostream>
#include <mutex>
namespace fpga {
namespace {
void emit(const Event& event) {
    static std::mutex mutex;
    std::lock_guard lock(mutex);
    std::cout << encodeEvent(event) << '\n' << std::flush;
}
}
int runWorker() {
    try {
        std::string input;
        char buffer[4096];
        while (std::cin) {
            std::cin.read(buffer, sizeof(buffer));
            input.append(buffer, static_cast<std::size_t>(std::cin.gcount()));
            if (input.size() > limits::requestBytes)
                fail("Worker request exceeds 64 KiB");
        }
        const auto request = decodeRequest(input);
        Context context{
            request.deadline(), {}, [](const std::string& log) { emit({log, {}, std::nullopt}); }};
        emit({{}, {}, perform(request, context)});
        return 0;
    } catch (const std::exception& error) {
        emit({{}, error.what(), std::nullopt});
        return 1;
    }
}
}
