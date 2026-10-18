#include "application/history.hpp"
#include "core/error.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <set>
namespace fpga {
void HistorySelection::reset(std::vector<HistoryEntry> devices) {
    devices_ = std::move(devices);
    visible_.clear();
    checked_.clear();
}
void HistorySelection::filter(std::string_view query, bool disconnectedOnly) {
    std::vector<HistoryEntry> visible;
    const auto needle = lower(std::string(query));
    for (const auto& device : devices_) {
        if (disconnectedOnly && device.present)
            continue;
        if (!needle.empty() &&
            lower(device.name + " " + device.deviceClass + " " + device.instance).find(needle) ==
                std::string::npos)
            continue;
        visible.push_back(device);
    }
    visible_ = std::move(visible);
    checked_.clear();
}
void HistorySelection::check(std::vector<std::size_t> rows) {
    std::erase_if(rows, [&](auto row) { return row >= visible_.size() || visible_[row].present; });
    std::sort(rows.begin(), rows.end());
    rows.erase(std::unique(rows.begin(), rows.end()), rows.end());
    checked_ = std::move(rows);
}
bool HistorySelection::allChecked() const {
    return !checked_.empty() &&
           checked_.size() == static_cast<std::size_t>(std::count_if(
                                  visible_.begin(), visible_.end(),
                                  [](const auto& device) { return !device.present; }));
}
void HistorySelection::toggleAll() {
    if (allChecked()) {
        checked_.clear();
        return;
    }
    std::vector<std::size_t> rows;
    for (std::size_t i = 0; i < visible_.size(); ++i)
        if (!visible_[i].present)
            rows.push_back(i);
    checked_ = std::move(rows);
}
RemoveHistoryRequest HistorySelection::removal() const {
    RemoveHistoryRequest request;
    for (const auto row : checked_)
        request.instances.push_back(visible_[row].instance);
    Request{request}.validate();
    return request;
}
void validateHistoryRemoval(const RemoveHistoryRequest& request,
                            std::span<const HistoryEntry> devices) {
    Request{request}.validate();
    std::set<std::string> disconnected, connected;
    for (const auto& device : devices)
        (device.present ? connected : disconnected).insert(lower(device.instance));
    for (const auto& instance : request.instances) {
        const auto id = lower(instance);
        if (!disconnected.contains(id) || connected.contains(id))
            fail("Device is connected or no longer in history: " + instance);
    }
}
HistoryRemovalReport removeHistoryInstances(const RemoveHistoryRequest& request,
                                            const std::function<bool(const std::string&)>& remove,
                                            const std::function<void()>& check) {
    Request{request}.validate();
    HistoryRemovalReport report;
    for (const auto& instance : request.instances) {
        try {
            check();
        } catch (const std::exception& error) {
            report.failures.push_back(error.what());
            break;
        }
        try {
            report.rebootRequired = remove(instance) || report.rebootRequired;
            report.removed.push_back(instance);
        } catch (const std::exception& error) {
            report.failures.push_back(instance + ": " + error.what());
        }
    }
    return report;
}
void HistoryRemovalReport::requireSuccess(std::size_t requested) const {
    if (failures.empty())
        return;
    fail("Removed " + std::to_string(removed.size()) + " of " + std::to_string(requested) +
         " device instances." + (rebootRequired ? " Restart Windows to finish." : "") + "\n" +
         join(failures, "\n"));
}
}
