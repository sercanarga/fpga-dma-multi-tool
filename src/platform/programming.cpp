#include "platform/service.hpp"
#include "platform/writer.hpp"
namespace fpga {
ProgramResult program(const ProgramRequest& request, Context& context) {
    request.firmware.validate();
    win::TemporaryDirectory staging;
    auto firmware = request.firmware;
    firmware.path =
        staging.path() /
        (request.firmware.destination == Destination::sram ? L"firmware.bit" : L"firmware.bin");
    std::filesystem::copy_file(request.firmware.path, firmware.path);
    firmware.validate();
    const auto hash = win::sha256(firmware.path);
    if (context.log)
        context.log("Firmware SHA-256: " + hash + "\n");
    auto writer = prepareWriter(request.connection, context);
    const auto device = scanWriter(context, writer, firmware.index).front();
    if (device.part != firmware.part)
        fail("Connected FPGA does not match selected model");
    if (request.expectedDna && *request.expectedDna != device.dna)
        fail("FPGA DNA changed since selection. Scan again.");
    std::vector<std::wstring> args{L"--fpga-part", win::wide(firmware.part), L"--index-chain",
                                   std::to_wstring(firmware.index)};
    if (firmware.destination == Destination::flash)
        args.insert(args.end(), {L"--write-flash", L"--verify"});
    args.push_back(firmware.path.native());
    loader(context, writer, args);
    return {device, hash, firmware.destination};
}
TableResult inspect(const InspectRequest& request, Context& context) {
    auto writer = prepareWriter(request.connection, context);
    const auto device = scanWriter(context, writer, request.index).front();
    if (request.expectedDna && *request.expectedDna != device.dna)
        fail("FPGA DNA changed. Scan again.");
    std::vector<std::wstring> base{L"--index-chain", std::to_wstring(request.index)};
    Rows rows;
    if (request.kind == Inspection::registers) {
        std::map<std::string, std::uint32_t> registers;
        for (const std::string name : {"STAT", "CTRL0", "BOOTSTS", "WBSTAR"}) {
            auto args = base;
            args.insert(args.end(), {L"--read-register", win::wide(name)});
            try {
                registers[name] = parseRegister(loader(context, writer, args));
            } catch (const std::exception& error) {
                context.check();
                rows.push_back({name, error.what()});
            }
        }
        if (registers.empty())
            fail("No FPGA configuration register could be read");
        auto decoded = configurationRows(registers);
        rows.insert(rows.end(), decoded.begin(), decoded.end());
    } else {
        auto args = base;
        if (request.kind == Inspection::sensors)
            args.push_back(L"--read-xadc");
        else
            args.insert(args.end(),
                        {L"--detect", L"--write-flash", L"--fpga-part", win::wide(device.part)});
        auto output = loader(context, writer, args);
        rows = request.kind == Inspection::sensors ? sensorRows(output) : flashRows(output);
    }
    return {{"Property", "Value"}, std::move(rows), "FPGA inspection completed"};
}
}
