#pragma once
#include "ui/page.hpp"
#include <array>
namespace fpga::ui {
enum class PageId : std::size_t { devices, flash, speed, history, system, drivers, count };
inline constexpr auto pageCount = static_cast<std::size_t>(PageId::count);
struct PageDefinition {
    const wchar_t* title;
    std::unique_ptr<Page> (*create)(PageHost&);
};
inline constexpr std::array pageDefinitions{
    PageDefinition{L"Devices", devicesPage},      PageDefinition{L"Flash", firmwarePage},
    PageDefinition{L"Speed Test", benchmarkPage}, PageDefinition{L"Device History", historyPage},
    PageDefinition{L"System Info", systemPage},   PageDefinition{L"Drivers", driversPage}};
static_assert(pageDefinitions.size() == pageCount);
}
