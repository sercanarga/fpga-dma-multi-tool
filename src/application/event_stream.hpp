#pragma once
#include "application/protocol.hpp"
#include <functional>
namespace fpga {
struct Completion {
    std::optional<Result> result;
    std::string error;
};
class EventStream {
    std::string pending_;
    Completion completion_;
    bool completed_ = false;
    std::function<void(std::string)> log_;
    void accept(std::string_view line);

  public:
    explicit EventStream(std::function<void(std::string)> log) : log_(std::move(log)) {}
    void append(std::string_view chunk);
    Completion finish(unsigned long exitCode);
};
}
