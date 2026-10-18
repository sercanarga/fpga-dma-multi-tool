#pragma once
#include "application/model.hpp"
#include "platform/process.hpp"
#include <optional>
namespace fpga {
struct WindowsDevice {
    std::string instance, name, deviceClass, enumerator, manufacturer, service, inf;
    bool present = false;
    std::optional<std::uint32_t> currentWidth, maximumWidth;
};
struct FtdiWriter {
    WindowsDevice device;
    std::string pid, serial;
    bool ready = false;
};
std::vector<WindowsDevice> windowsDevices(Context& context, bool presentOnly = false);
std::vector<FtdiWriter> ftdiWriters(Context& context);
std::optional<std::string> writerPid(std::string_view instance);
HistoryResult deviceHistory(Context& context);
bool removeDevices(const RemoveHistoryRequest& request, Context& context);
void rescanDevices();
}
