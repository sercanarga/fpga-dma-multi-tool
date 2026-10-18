#include "platform/service.hpp"
#include "platform/devices.hpp"
#include "platform/drivers.hpp"
#include "platform/writer.hpp"
#include <type_traits>
namespace fpga {
Result inventory(const InventoryRequest& request, Context& context) {
    switch (request.kind) {
    case Inventory::history:
        return deviceHistory(context);
    case Inventory::drivers:
        return driverStatus(context);
    case Inventory::system:
        return systemInformation(context);
    }
    fail("Invalid inventory operation");
}
Result perform(const Request& request, Context& context) {
    request.validate();
    context.check();
    return std::visit(
        [&](const auto& operation) -> Result {
            using T = std::decay_t<decltype(operation)>;
            if constexpr (std::is_same_v<T, ScanRequest>) {
                auto writer = prepareWriter(operation.connection, context);
                return ScanResult{scanWriter(context, writer, operation.index)};
            } else if constexpr (std::is_same_v<T, ProgramRequest>)
                return program(operation, context);
            else if constexpr (std::is_same_v<T, InspectRequest>)
                return inspect(operation, context);
            else if constexpr (std::is_same_v<T, BenchmarkRequest>)
                return benchmark(operation, context);
            else if constexpr (std::is_same_v<T, InventoryRequest>)
                return inventory(operation, context);
            else
                return manageWindows(request, context);
        },
        request.operation);
}
}
