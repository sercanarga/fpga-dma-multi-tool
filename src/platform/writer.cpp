#include "platform/writer.hpp"
#include "core/text.hpp"
#include "platform/devices.hpp"
#include "platform/socket.hpp"
#include <algorithm>
namespace fpga {
std::string loader(Context& context, Writer& writer, const std::vector<std::wstring>& arguments) {
    std::unique_ptr<Bridge> bridge;
    auto args = writer.arguments;
    if (writer.backend == "ch347") {
        bridge = std::make_unique<Bridge>(*writer.transport, context);
        args = {L"--cable",   L"xvc-client", L"--ip",
                L"127.0.0.1", L"--port",     std::to_wstring(bridge->port())};
    }
    args.insert(args.end(), arguments.begin(), arguments.end());
    ProcessOptions options;
    options.executable = writer.executable;
    options.arguments = std::move(args);
    options.directory = writer.executable.parent_path();
    options.environment[L"OPENFPGALOADER_SOJ_DIR"] = (options.directory / L"data").native();
    const DWORD needed = GetEnvironmentVariableW(L"PATH", nullptr, 0);
    std::wstring path(needed, L'\0');
    if (needed) {
        GetEnvironmentVariableW(L"PATH", path.data(), needed);
        path.resize(needed - 1);
    }
    options.environment[L"PATH"] = options.directory.native() + L';' + path;
    try {
        auto result = execute(context, options);
        if (bridge)
            bridge->check();
        return result.output;
    } catch (...) {
        if (bridge)
            bridge->check();
        throw;
    }
}
namespace {
Writer ftdiWriter(const Connection& connection, Context& context) {
    Writer writer;
    writer.executable = win::runtimePath("openFPGALoader/openFPGALoader-ftdi.exe");
    writer.backend = "ftdi";
    auto ready = ftdiWriters(context);
    std::erase_if(ready, [](const auto& item) { return !item.ready; });
    const auto total = ready.size();
    if (!connection.serial.empty())
        std::erase_if(ready, [&](const auto& row) { return row.serial != connection.serial; });
    if (ready.empty())
        fail("No ready FTDI Interface A writer. Check the selected serial and WinUSB driver.");
    if (ready.size() != 1)
        fail("Select the serial number of exactly one FTDI writer");
    const auto& device = ready.front();
    const auto description = lower(device.device.name + device.device.manufacturer);
    std::vector<std::wstring> cables;
    if (device.pid == "6010")
        cables = description.find("digilent") != description.npos
                     ? std::vector<std::wstring>{L"digilent", L"rs_dma", L"ft2232"}
                     : std::vector<std::wstring>{L"ft2232", L"rs_dma", L"digilent"};
    if (device.pid == "6011")
        cables = {L"rs_dma", L"ft4232"};
    if (device.pid == "6014") {
        cables = {L"ft232", L"rs_dma", L"digilent_hs3", L"digilent_hs2", L"digilent_ad"};
        if (description.find("hs3") != description.npos)
            std::rotate(cables.begin(), cables.begin() + 2, cables.begin() + 3);
        else if (description.find("hs2") != description.npos ||
                 description.find("smt2") != description.npos)
            std::rotate(cables.begin(), cables.begin() + 3, cables.begin() + 4);
    }
    std::vector<std::wstring> base{
        L"--vid", L"0x0403", L"--pid", win::wide("0x" + device.pid), L"--ftdi-channel", L"0"};
    if (!device.serial.empty() && device.serial.find_first_of("&\\") == device.serial.npos) {
        base.push_back(L"--ftdi-serial");
        base.push_back(win::wide(device.serial));
    } else if (total != 1)
        fail("FTDI writer has no unambiguous serial number");
    std::vector<std::string> errors;
    for (const auto& cable : cables) {
        writer.arguments = base;
        writer.arguments.insert(writer.arguments.end(), {L"--cable", cable});
        try {
            parseChain(loader(context, writer, {L"--detect"}));
            writer.target = device.device.instance + " / " + win::utf8(cable);
            return writer;
        } catch (const std::exception& error) {
            context.check();
            errors.push_back(win::utf8(cable) + ": " + error.what());
        }
    }
    fail("No FTDI cable profile read the FPGA\n" + join(errors, "\n"));
}
}
Writer prepareWriter(const Connection& connection, Context& context) {
    connection.validate();
    Writer writer;
    if (connection.backend == Backend::automatic || connection.backend == Backend::ch347) {
        try {
            writer.transport = openCh347(connection, context);
            writer.executable = win::runtimePath("openFPGALoader/openFPGALoader.exe");
            writer.backend = "ch347";
            writer.target = connection.adapter < 0
                                ? "CH347"
                                : "CH347[" + std::to_string(connection.adapter) + "]";
            return writer;
        } catch (const NoAdapter&) {
            if (connection.backend == Backend::ch347)
                throw;
        }
    }
    return ftdiWriter(connection, context);
}
std::vector<Device> scanWriter(Context& context, Writer& writer, int selected) {
    if (writer.transport)
        return Jtag(*writer.transport).scan(selected, writer.target, writer.backend);
    auto chain = parseChain(loader(context, writer, {L"--detect"}));
    if (selected < -1 || selected >= static_cast<int>(chain.size()))
        fail("Selected device is outside the JTAG chain");
    std::vector<Device> result;
    for (int i = 0; i < static_cast<int>(chain.size()); ++i) {
        if (selected >= 0 && selected != i)
            continue;
        std::vector<std::wstring> args{L"--read-dna", L"--index-chain", std::to_wstring(i)};
        auto dna = parseDna(loader(context, writer, args));
        if (dna != parseDna(loader(context, writer, args)))
            fail("Unstable DNA. Check cable and power.");
        auto id = chain[static_cast<std::size_t>(i)];
        auto part = partName(id);
        result.push_back({i, part, writer.target, writer.backend, id, dna,
                          dna & ((std::uint64_t{1} << 57) - 1), boardMatches(part, chain)});
    }
    return result;
}
}
