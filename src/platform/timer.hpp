#pragma once
#include "platform/windows.hpp"
#include <chrono>
namespace fpga::win {
class ShortWait {
    Handle timer_;
    LARGE_INTEGER due_{};

  public:
    explicit ShortWait(std::chrono::microseconds duration);
    void wait();
};
}
