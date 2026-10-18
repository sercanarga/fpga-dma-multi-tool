#pragma once
#include "core/log.hpp"
#include "platform/windows.hpp"
#include <chrono>
#include <functional>
#include <map>
#include <span>
#include <stop_token>
#include <vector>
namespace fpga {
struct Context {
    std::chrono::steady_clock::time_point deadline;
    std::stop_token stop;
    Log log;
    void check() const;
};
using Environment = std::map<std::wstring, std::wstring>;
struct ProcessOptions {
    std::filesystem::path executable;
    std::vector<std::wstring> arguments;
    Environment environment;
    std::filesystem::path directory;
    std::string input;
    bool allowFailure = false;
    bool captureOutput = true;
};
struct ProcessResult {
    DWORD exitCode;
    std::string output;
};
std::wstring quoteArgument(std::wstring_view argument);
ProcessResult execute(Context& context, const ProcessOptions& options);
}
