#pragma once
#include <functional>
#include <string>
namespace fpga {
using Log = std::function<void(const std::string&)>;
}
