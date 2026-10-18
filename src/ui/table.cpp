#include "ui/table.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <numeric>
#include <unordered_set>
namespace fpga::ui {
void Table::create(HWND parent, int id, const Row& columns) {
    columns_ = columns;
    window_ = control(parent, WC_LISTVIEWW, L"",
                      WS_TABSTOP | WS_CLIPCHILDREN | LVS_REPORT | LVS_SINGLESEL | LVS_SHOWSELALWAYS,
                      id, WS_EX_CLIENTEDGE);
    ListView_SetExtendedListViewStyle(window_,
                                      LVS_EX_FULLROWSELECT | LVS_EX_DOUBLEBUFFER | LVS_EX_LABELTIP);
    int index = 0;
    for (const auto& name : columns) {
        auto label = win::wide(name);
        LVCOLUMNW column{};
        column.mask = LVCF_TEXT | LVCF_WIDTH;
        column.pszText = label.data();
        column.cx = scaled(parent, index == 0 && name == "#" ? 42 : 130);
        SendMessageW(window_, LVM_INSERTCOLUMNW, static_cast<WPARAM>(index++),
                     reinterpret_cast<LPARAM>(&column));
    }
}
void Table::set(Rows rows) {
    groups_.clear();
    rowGroups_.clear();
    rows_ = std::move(rows);
    order_.resize(rows_.size());
    std::iota(order_.begin(), order_.end(), 0);
    sortColumn_ = -1;
    render();
}
void Table::setGroups(std::vector<std::pair<std::wstring, Rows>> groups) {
    groups_.clear();
    rowGroups_.clear();
    rows_.clear();
    for (auto& [title, rows] : groups) {
        if (rows.empty())
            continue;
        const auto index = static_cast<int>(groups_.size());
        groups_.push_back(std::move(title));
        for (auto& row : rows) {
            rows_.push_back(std::move(row));
            rowGroups_.push_back(index);
        }
    }
    order_.resize(rows_.size());
    std::iota(order_.begin(), order_.end(), 0);
    sortColumn_ = -1;
    render();
}
LRESULT Table::draw(NMLVCUSTOMDRAW& draw) const {
    if (statusColumn_ < 0 ||
        (draw.dwItemType == LVCDI_GROUP && draw.nmcd.dwDrawStage != CDDS_PREPAINT))
        return CDRF_DODEFAULT;
    switch (draw.nmcd.dwDrawStage) {
    case CDDS_PREPAINT:
        return CDRF_NOTIFYITEMDRAW;
    case CDDS_ITEMPREPAINT:
        return CDRF_NOTIFYSUBITEMDRAW;
    case CDDS_ITEMPREPAINT | CDDS_SUBITEM: {
        const auto index = static_cast<std::size_t>(draw.nmcd.dwItemSpec);
        if (index >= order_.size() || !IsWindowEnabled(window_) ||
            (ListView_GetItemState(window_, static_cast<int>(index), LVIS_SELECTED) &
             LVIS_SELECTED))
            return CDRF_DODEFAULT;
        draw.clrText = GetSysColor(COLOR_WINDOWTEXT);
        if (draw.iSubItem != statusColumn_)
            return CDRF_NEWFONT;
        const auto& row = rows_[order_[index]];
        const auto column = static_cast<std::size_t>(statusColumn_);
        if (column >= row.size())
            return CDRF_DODEFAULT;
        const auto& value = row[column];
        StatusTone tone = StatusTone::neutral;
        if (value == "Yes" || value == "On" || value == "Enabled" || value == "Running" ||
            value == "Available" || value == "Ready" || value == "UEFI")
            tone = StatusTone::positive;
        else if (value == "No" || value == "Off" || value == "Disabled" || value == "Not running" ||
                 value == "Not present" || value == "Not ready" ||
                 value == "Enabled, not running" || value == "Legacy BIOS")
            tone = StatusTone::negative;
        if (const auto color = statusColor(tone, GetSysColor(COLOR_WINDOW))) {
            draw.clrText = *color;
            return CDRF_NEWFONT;
        }
        return CDRF_NEWFONT;
    }
    default:
        return CDRF_DODEFAULT;
    }
}
void Table::render() {
    SendMessageW(window_, WM_SETREDRAW, FALSE, 0);
    ListView_DeleteAllItems(window_);
    ListView_RemoveAllGroups(window_);
    ListView_EnableGroupView(window_, !groups_.empty());
    for (std::size_t i = 0; i < groups_.size(); ++i) {
        LVGROUP group{};
        group.cbSize = sizeof(group);
        group.mask = LVGF_HEADER | LVGF_GROUPID | LVGF_STATE;
        group.pszHeader = groups_[i].data();
        group.iGroupId = static_cast<int>(i);
        group.stateMask = LVGS_COLLAPSIBLE;
        group.state = LVGS_COLLAPSIBLE;
        ListView_InsertGroup(window_, -1, &group);
    }
    int index = 0;
    for (auto original : order_) {
        const auto& row = rows_[original];
        for (std::size_t column = 0; column < row.size(); ++column) {
            auto value = win::wide(row[column]);
            LVITEMW item{};
            item.mask = LVIF_TEXT | LVIF_PARAM;
            if (!groups_.empty()) {
                item.mask |= LVIF_GROUPID;
                item.iGroupId = rowGroups_[original];
            }
            item.iItem = index;
            item.iSubItem = static_cast<int>(column);
            item.pszText = value.data();
            item.lParam = static_cast<LPARAM>(original);
            SendMessageW(window_, column ? LVM_SETITEMTEXTW : LVM_INSERTITEMW,
                         column ? static_cast<WPARAM>(index) : 0, reinterpret_cast<LPARAM>(&item));
        }
        ++index;
    }
    SendMessageW(window_, WM_SETREDRAW, TRUE, 0);
    InvalidateRect(window_, nullptr, TRUE);
    resizeColumns();
}
void Table::resizeColumns() {
    if (!groups_.empty() && columns_.size() == 2) {
        RECT area{};
        GetClientRect(window_, &area);
        const auto labelWidth = std::min(scaled(window_, 280), static_cast<int>(area.right) / 2);
        ListView_SetColumnWidth(window_, 0, labelWidth);
        ListView_SetColumnWidth(window_, 1, std::max(0L, area.right - labelWidth - 4));
        return;
    }
    auto header = ListView_GetHeader(window_);
    const auto count = Header_GetItemCount(header);
    for (int column = 0; column < count; ++column) {
        if (column == 0 && !columns_.empty() && columns_[0].empty()) {
            ListView_SetColumnWidth(window_, column, scaled(window_, 32));
            continue;
        }
        if (column == 0 && !columns_.empty() && columns_[0] == "#") {
            ListView_SetColumnWidth(window_, column, scaled(window_, 42));
            continue;
        }
        ListView_SetColumnWidth(window_, column, LVSCW_AUTOSIZE_USEHEADER);
        auto width = ListView_GetColumnWidth(window_, column);
        ListView_SetColumnWidth(window_, column,
                                std::clamp(width, scaled(window_, 90), scaled(window_, 480)));
    }
}
void Table::sort(int column) {
    if (column < 0 || static_cast<std::size_t>(column) >= columns_.size())
        return;
    const auto previous = selected();
    const auto checks = checkedRows();
    std::vector<UINT> groupStates;
    for (std::size_t i = 0; i < groups_.size(); ++i)
        groupStates.push_back(ListView_GetGroupState(window_, static_cast<int>(i), LVGS_COLLAPSED));
    std::vector<std::string> keys;
    keys.reserve(rows_.size());
    for (const auto& row : rows_)
        keys.push_back(static_cast<std::size_t>(column) < row.size()
                           ? lower(row[static_cast<std::size_t>(column)])
                           : std::string{});
    ascending_ = sortColumn_ == column ? !ascending_ : true;
    sortColumn_ = column;
    std::stable_sort(order_.begin(), order_.end(), [&](auto a, auto b) {
        if (!groups_.empty() && rowGroups_[a] != rowGroups_[b])
            return rowGroups_[a] < rowGroups_[b];
        return ascending_ ? keys[a] < keys[b] : keys[a] > keys[b];
    });
    render();
    for (std::size_t i = 0; i < groupStates.size(); ++i)
        ListView_SetGroupState(window_, static_cast<int>(i), LVGS_COLLAPSED, groupStates[i]);
    checkRows(checks);
    if (previous) {
        const auto found = std::find(order_.begin(), order_.end(), *previous);
        if (found != order_.end())
            ListView_SetItemState(window_, static_cast<int>(found - order_.begin()),
                                  LVIS_SELECTED | LVIS_FOCUSED, LVIS_SELECTED | LVIS_FOCUSED);
    }
}
std::optional<std::size_t> Table::selected() const {
    int row = ListView_GetNextItem(window_, -1, LVNI_SELECTED);
    if (row < 0 || static_cast<std::size_t>(row) >= order_.size())
        return std::nullopt;
    return order_[static_cast<std::size_t>(row)];
}
std::vector<std::size_t> Table::checkedRows() const {
    std::vector<std::size_t> rows;
    if (!(ListView_GetExtendedListViewStyle(window_) & LVS_EX_CHECKBOXES))
        return rows;
    for (std::size_t i = 0; i < order_.size(); ++i)
        if (ListView_GetCheckState(window_, static_cast<int>(i)))
            rows.push_back(order_[i]);
    return rows;
}
void Table::checkRows(const std::vector<std::size_t>& rows) {
    if (!(ListView_GetExtendedListViewStyle(window_) & LVS_EX_CHECKBOXES))
        return;
    const std::unordered_set<std::size_t> selected(rows.begin(), rows.end());
    for (std::size_t i = 0; i < order_.size(); ++i)
        ListView_SetCheckState(window_, static_cast<int>(i), selected.contains(order_[i]));
}
void Table::copy(HWND owner) const {
    const auto row = selected();
    if (!row)
        return;
    Row values;
    for (std::size_t i = 0; i < rows_[*row].size() && i < columns_.size(); ++i)
        if (!columns_[i].empty())
            values.push_back(rows_[*row][i]);
    clipboard(owner, win::wide(join(values, "\t")));
}
}
