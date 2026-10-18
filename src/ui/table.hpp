#pragma once
#include "core/inspection.hpp"
#include "ui/controls.hpp"
namespace fpga::ui {
class Table {
    HWND window_ = nullptr;
    Rows rows_;
    Row columns_;
    std::vector<std::size_t> order_;
    int sortColumn_ = -1;
    bool ascending_ = true;
    int statusColumn_ = -1;
    std::vector<std::wstring> groups_;
    std::vector<int> rowGroups_;
    void render();

  public:
    void create(HWND parent, int id, const Row& columns);
    HWND window() const {
        return window_;
    }
    void set(Rows rows);
    void setGroups(std::vector<std::pair<std::wstring, Rows>> groups);
    void colorStatusColumn(int column) {
        statusColumn_ = column;
    }
    LRESULT draw(NMLVCUSTOMDRAW& draw) const;
    void sort(int column);
    std::optional<std::size_t> selected() const;
    std::vector<std::size_t> checkedRows() const;
    void checkRows(const std::vector<std::size_t>& rows);
    void copy(HWND owner) const;
    void resizeColumns();
    const Row& columns() const {
        return columns_;
    }
    const Rows& rows() const {
        return rows_;
    }
};
}
