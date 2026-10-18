#pragma once
#include <cstddef>
namespace fpga::limits {
inline constexpr std::size_t requestBytes = 64 * 1024;
inline constexpr std::size_t eventBytes = 16 * 1024 * 1024;
inline constexpr std::size_t diagnosticBytes = 16 * 1024 * 1024;
}
