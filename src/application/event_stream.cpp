#include "application/event_stream.hpp"
#include "application/limits.hpp"
#include "core/error.hpp"
namespace fpga {
void EventStream::accept(std::string_view line) {
    if (completed_)
        fail("Worker sent output after completion");
    auto event = decodeEvent(line);
    if (event.result) {
        completion_.result = std::move(event.result);
        completed_ = true;
    } else if (!event.error.empty()) {
        completion_.error = std::move(event.error);
        completed_ = true;
    } else if (log_)
        log_(std::move(event.log));
}
void EventStream::append(std::string_view chunk) {
    while (!chunk.empty()) {
        const auto end = chunk.find('\n');
        const auto length = end == chunk.npos ? chunk.size() : end;
        if (length > limits::eventBytes - pending_.size())
            fail("Worker event exceeds limit");
        pending_.append(chunk.substr(0, length));
        if (end == chunk.npos)
            return;
        accept(pending_);
        pending_.clear();
        chunk.remove_prefix(length + 1);
    }
}
Completion EventStream::finish(unsigned long exitCode) {
    if (!pending_.empty())
        fail("Truncated worker event");
    if (!completed_ || (exitCode && completion_.error.empty()))
        fail("Worker exited before completing the operation (exit " + std::to_string(exitCode) +
             ")");
    return std::move(completion_);
}
}
