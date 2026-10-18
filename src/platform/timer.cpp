#include "platform/timer.hpp"
namespace fpga::win {
ShortWait::ShortWait(std::chrono::microseconds duration) {
    if (duration <= std::chrono::microseconds::zero() || duration > std::chrono::seconds(1))
        fail("Invalid short wait duration");
    auto timer = CreateWaitableTimerExW(nullptr, nullptr, CREATE_WAITABLE_TIMER_HIGH_RESOLUTION,
                                        TIMER_MODIFY_STATE | SYNCHRONIZE);
    if (!timer && GetLastError() == ERROR_INVALID_PARAMETER)
        timer = CreateWaitableTimerExW(nullptr, nullptr, 0, TIMER_MODIFY_STATE | SYNCHRONIZE);
    timer_ = Handle(timer);
    if (!timer_)
        fail("Cannot create transfer pacing timer: " + error());
    due_.QuadPart = -duration.count() * 10;
}
void ShortWait::wait() {
    if (!SetWaitableTimerEx(timer_.get(), &due_, 0, nullptr, nullptr, nullptr, 0))
        fail("Cannot arm transfer pacing timer: " + error());
    if (WaitForSingleObject(timer_.get(), 1500) != WAIT_OBJECT_0)
        fail("Transfer pacing timer did not complete");
}
}
