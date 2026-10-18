#pragma once
#include <stdexcept>
#include <string>
namespace fpga {
[[noreturn]] inline void fail(const std::string& message) {
    throw std::runtime_error(message);
}
}
