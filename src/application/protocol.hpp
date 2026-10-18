#pragma once
#include "application/model.hpp"
#include <string_view>
namespace fpga {
std::string encodeRequest(const Request& request);
Request decodeRequest(std::string_view data);
std::string encodeResult(const Result& result);
Result decodeResult(std::string_view data);
struct Event {
    std::string log, error;
    std::optional<Result> result;
};
std::string encodeEvent(const Event& event);
Event decodeEvent(std::string_view data);
}
