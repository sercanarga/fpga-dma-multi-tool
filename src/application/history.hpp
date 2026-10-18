#pragma once
#include "application/model.hpp"
#include <functional>
#include <span>
namespace fpga {
class HistorySelection {
    std::vector<HistoryEntry> devices_, visible_;
    std::vector<std::size_t> checked_;

  public:
    void reset(std::vector<HistoryEntry> devices);
    void filter(std::string_view query, bool disconnectedOnly);
    void check(std::vector<std::size_t> rows);
    void toggleAll();
    bool allChecked() const;
    RemoveHistoryRequest removal() const;
    const std::vector<HistoryEntry>& visible() const {
        return visible_;
    }
    const std::vector<std::size_t>& checked() const {
        return checked_;
    }
};
void validateHistoryRemoval(const RemoveHistoryRequest& request,
                            std::span<const HistoryEntry> devices);
struct HistoryRemovalReport {
    std::vector<std::string> removed;
    std::vector<std::string> failures;
    bool rebootRequired = false;
    void requireSuccess(std::size_t requested) const;
};
HistoryRemovalReport removeHistoryInstances(const RemoveHistoryRequest& request,
                                            const std::function<bool(const std::string&)>& remove,
                                            const std::function<void()>& check);
}
