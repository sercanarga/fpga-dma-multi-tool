#pragma once
#include "application/model.hpp"
#include "platform/process.hpp"
namespace fpga {
Result perform(const Request& request, Context& context);
ProgramResult program(const ProgramRequest& request, Context& context);
TableResult inspect(const InspectRequest& request, Context& context);
BenchmarkResult benchmark(const BenchmarkRequest& request, Context& context);
Result inventory(const InventoryRequest& request, Context& context);
TableResult systemInformation(Context& context);
}
