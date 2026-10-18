#pragma once
#include "application/event_stream.hpp"
#include "platform/windows.hpp"
#include <deque>
#include <mutex>
#include <thread>
namespace fpga {
using Update = std::variant<std::string, Completion>;
class WorkQueue {
    HWND receiver_;
    std::mutex mutex_;
    std::deque<Update> updates_;
    std::size_t logBytes_ = 0;
    bool running_ = false;
    std::jthread thread_;
    void publish(Update update);

  public:
    static constexpr UINT message = WM_APP + 1;
    explicit WorkQueue(HWND receiver) : receiver_(receiver) {}
    ~WorkQueue();
    WorkQueue(const WorkQueue&) = delete;
    WorkQueue& operator=(const WorkQueue&) = delete;
    void start(const Request& request);
    void cancel();
    std::deque<Update> take();
};
}
