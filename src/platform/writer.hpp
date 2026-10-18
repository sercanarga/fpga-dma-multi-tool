#pragma once
#include "application/model.hpp"
#include "core/jtag.hpp"
#include "platform/process.hpp"
#include <memory>
namespace fpga {
class NoAdapter final : public std::runtime_error {
  public:
    NoAdapter() : std::runtime_error("No CH347 adapter connected") {};
};
std::unique_ptr<Transport> openCh347(const Connection& connection, Context& context);
struct Writer {
    std::filesystem::path executable;
    std::string backend, target;
    std::vector<std::wstring> arguments;
    std::unique_ptr<Transport> transport;
};
Writer prepareWriter(const Connection& connection, Context& context);
std::string loader(Context& context, Writer& writer, const std::vector<std::wstring>& arguments);
std::vector<Device> scanWriter(Context& context, Writer& writer, int selected);
}
