#include "ui/window.hpp"
namespace fpga::ui {
void Window::submit(Request request) {
    if (!options_.operationsEnabled)
        fail("Operations are disabled in this preview");
    if (active_)
        fail("An operation is already running");
    request.validate();
    const auto page = TabCtrl_GetCurSel(tabs_);
    if (page < 0 || static_cast<std::size_t>(page) >= pages_.size())
        fail("No active page for this operation");
    active_ = ActiveOperation{std::move(request), static_cast<std::size_t>(page)};
    try {
        work_->start(active_->request);
    } catch (...) {
        active_.reset();
        throw;
    }
    setBusy(true);
    SendMessageW(status_, SB_SETTEXTW, 0, reinterpret_cast<LPARAM>(L"Working..."));
}
void Window::setBusy(bool busy) {
    SendMessageW(progress_, PBM_SETMARQUEE, busy, 30);
    ShowWindow(progress_, busy ? SW_SHOW : SW_HIDE);
    if (busy)
        operationFocus_ = GetFocus();
    for (std::size_t i = 0; i < pages_.size(); ++i)
        pages_[i]->setBusy(busy,
                           busy && active_ && i == active_->page && !active_->request.privileged(),
                           busy && active_ && i == active_->page);
    layout();
    if (!busy && IsWindow(operationFocus_) && IsWindowVisible(operationFocus_) &&
        IsWindowEnabled(operationFocus_))
        SetFocus(operationFocus_);
}
void Window::drain() {
    if (!work_)
        return;
    for (auto& update : work_->take()) {
        if (!active_)
            fail("Worker update has no active operation");
        const auto source = active_->page;
        if (auto* log = std::get_if<std::string>(&update)) {
            presentLog(static_cast<PageId>(source), *log);
            continue;
        }
        auto completion = std::move(std::get<Completion>(update));
        active_.reset();
        setBusy(false);
        try {
            if (!completion.error.empty())
                fail(completion.error);
            if (!completion.result)
                fail("Worker completed without a result");
            present(static_cast<PageId>(source), std::move(*completion.result));
        } catch (const std::exception& error) {
            pages_[source]->showError(error.what());
            status(error.what());
        }
        const auto current = TabCtrl_GetCurSel(tabs_);
        if (!active_ && current >= 0 && static_cast<std::size_t>(current) != source)
            selectTab(current);
    }
}
void Window::cancel() {
    if (!active_ || active_->request.privileged())
        return;
    if (active_->request.programming() &&
        !confirm(window_, L"Interrupt programming? The FPGA or flash may be left incomplete."))
        return;
    work_->cancel();
}
void Window::exportResult() {
    const auto current = TabCtrl_GetCurSel(tabs_);
    if (current < 0 || static_cast<std::size_t>(current) >= results_.size() ||
        !results_[static_cast<std::size_t>(current)])
        fail("There is no report to export for this page");
    if (auto path = chooseFile(window_, true))
        win::atomicWrite(*path, encodeResult(*results_[static_cast<std::size_t>(current)]) + "\n");
}
void Window::present(PageId page, Result result) {
    const auto index = static_cast<std::size_t>(page);
    auto& stored = results_.at(index);
    stored = std::move(result);
    const auto message = resultMessage(*stored);
    status(message);
    pages_.at(index)->appendLog(message + "\n");
    pages_.at(index)->complete(*stored);
}
void Window::presentLog(PageId page, const std::string& message) {
    pages_.at(static_cast<std::size_t>(page))->appendLog(message);
}
}
