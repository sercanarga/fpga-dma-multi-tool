#include "application/work_queue.hpp"
#include "application/limits.hpp"
#include "platform/process.hpp"
namespace fpga {
WorkQueue::~WorkQueue() {
    cancel();
    if (thread_.joinable())
        thread_.join();
}
void WorkQueue::cancel() {
    thread_.request_stop();
}
void WorkQueue::publish(Update update) {
    bool notify = false;
    {
        std::lock_guard lock(mutex_);
        if (const auto* text = std::get_if<std::string>(&update)) {
            if (text->size() > limits::diagnosticBytes - logBytes_)
                fail("Pending diagnostic output exceeds 16 MiB");
            logBytes_ += text->size();
        } else
            running_ = false;
        notify = updates_.empty();
        updates_.push_back(std::move(update));
    }
    if (notify)
        PostMessageW(receiver_, message, 0, 0);
}
std::deque<Update> WorkQueue::take() {
    std::lock_guard lock(mutex_);
    std::deque<Update> result;
    result.swap(updates_);
    logBytes_ = 0;
    return result;
}
void WorkQueue::start(const Request& request) {
    request.validate();
    const auto input = encodeRequest(request);
    {
        std::lock_guard lock(mutex_);
        if (running_ || !updates_.empty())
            fail("The previous worker operation has not completed");
        running_ = true;
    }
    try {
        if (thread_.joinable())
            thread_.join();
        thread_ = std::jthread([this, request, input](std::stop_token token) {
            Completion completion;
            try {
                EventStream events([&](std::string log) { publish(std::move(log)); });
                Context context{request.deadline(std::chrono::seconds(3)), token,
                                [&](const std::string& chunk) { events.append(chunk); }};
                ProcessOptions options;
                options.executable = win::executablePath();
                options.arguments = {L"--worker"};
                options.input = input;
                options.allowFailure = true;
                options.captureOutput = false;
                completion = events.finish(execute(context, options).exitCode);
            } catch (const std::exception& error) {
                completion.result.reset();
                completion.error = token.stop_requested() ? "Operation cancelled" : error.what();
            } catch (...) {
                completion = {{}, "Unexpected worker failure"};
            }
            publish(std::move(completion));
        });
    } catch (...) {
        std::lock_guard lock(mutex_);
        running_ = false;
        throw;
    }
}
}
