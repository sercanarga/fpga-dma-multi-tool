#include "application/protocol.hpp"
#include "core/jtag.hpp"
#include "core/text.hpp"
#include "test.hpp"
#include <algorithm>
#include <limits>
#include <set>
namespace {
class Chain final : public fpga::Transport {
  public:
    bool unstable = false;
    bool unsupported = false;
    int reads = 0, selected = 0;
    std::vector<std::uint32_t> ids{0x13631093};
    fpga::Bits shift(const fpga::Bits& tms, const fpga::Bits& tdi) override {
        fpga::Bits out(tms.size());
        if (tdi.size() == 549) {
            for (std::size_t i = 0; i < ids.size(); ++i) {
                const auto id = fpga::bits(unsupported ? 0x12345679 : ids[i], 32);
                std::copy(id.begin(), id.end(),
                          out.begin() + static_cast<std::ptrdiff_t>(3 + i * 32));
            }
        } else if (tdi.size() == ids.size() * 6 + 6 && tms[1]) {
            for (int i = 0; i < static_cast<int>(ids.size()); ++i)
                if (fpga::readBits(tdi, 4 + i * 6, 6) == 0x32)
                    selected = i;
        } else if (tdi.size() == 68 + ids.size()) {
            const auto dna = fpga::bits(0x0011223344556677ULL + static_cast<unsigned>(selected) +
                                            (unstable ? reads++ : 0),
                                        64);
            std::copy(dna.begin(), dna.end(), out.begin() + 3 + selected);
        }
        return out;
    }
};
}

int main() {
    using namespace fpga;
    using test::rejects;
    return test::run(
        {{"stable and selected JTAG chains",
          [] {
              Chain chain;
              auto devices = Jtag(chain).scan(-1, "test", "fake");
              CHECK(devices.size() == 1);
              CHECK(devices[0].part == "xc7a100t");
              CHECK(devices[0].fuse == 0x0011223344556677ULL);
              CHECK(devices[0].dna == deviceDna(devices[0].fuse));
              chain.ids = {0x13632093, 0x13631093};
              devices = Jtag(chain).scan(-1, "test", "fake");
              CHECK(devices.size() == 2);
              CHECK(devices[1].fuse == 0x0011223344556678ULL);
              CHECK(Jtag(chain).scan(1, "test", "fake").front().index == 1);
          }},
         {"unstable, unsupported and out-of-range devices",
          [] {
              Chain chain;
              chain.unstable = true;
              rejects([&] { Jtag(chain).scan(-1, "test", "fake"); });
              chain.unstable = false;
              chain.unsupported = true;
              rejects([&] { Jtag(chain).scan(-1, "test", "fake"); });
              chain.unsupported = false;
              rejects([&] { Jtag(chain).scan(1, "test", "fake"); });
          }},
         {"DNA and CH347 wire contract",
          [] {
              CHECK(deviceDna(0x0123456789abcdefULL) == 0x01b3d591e6a2c480ULL);
              CHECK(ch347Command({0, 1}, {1, 0}) ==
                    Bytes({0xd2, 5, 0, 0x30, 0x31, 0x22, 0x23, 0x22}));
              CHECK(ch347Response(Bytes{0xd2, 2, 0, 1, 3}, 2) == Bits({1, 1}));
              rejects([] { ch347Response(Bytes{0xd2, 2, 0, 1}, 2); });
              rejects([] { ch347Command(Bits(249), Bits(249)); });
          }},
         {"packed JTAG bits",
          [] {
              Bits input{1, 0, 1, 0, 0, 0, 0, 1, 1};
              CHECK(pack(input) == Bytes({0x85, 1}));
              CHECK(unpack(pack(input), 9) == input);
              rejects([] { unpack({}, 1); });
              rejects([] { readBits({1}, 0, 65); });
          }},
         {"loader response parsing",
          [] {
              CHECK(parseChain("index 0:\n idcode 0x13631093\n")[0] == 0x13631093U);
              CHECK(parseDna(R"({"dna":"0011223344556677"})") == 0x0011223344556677ULL);
              rejects([] { parseDna(R"({"dna":"0"})"); });
              rejects([] { parseChain("index 2:\n idcode 0x03631093\n"); });
          }},
         {"firmware target and payload validation",
          [] {
              Bytes data{0, 9, 0xf, 0xf0, 0xf, 0xf0, 0xf, 0xf0, 0xf, 0xf0, 0, 0, 1};
              for (char key : {'a', 'b', 'c', 'd'}) {
                  std::string value = key == 'b' ? "7a100tcsg324" : "x";
                  value += '\0';
                  data.push_back(static_cast<std::uint8_t>(key));
                  data.push_back(0);
                  data.push_back(static_cast<std::uint8_t>(value.size()));
                  data.insert(data.end(), value.begin(), value.end());
              }
              const Bytes payload{'e', 0, 0, 0, 4, 0xaa, 0x99, 0x55, 0x66};
              data.insert(data.end(), payload.begin(), payload.end());
              validateImage(data, Destination::sram, "xc7a100t");
              rejects([&] { validateImage(data, Destination::sram, "xc7a75t"); });
              data.pop_back();
              rejects([&] { validateImage(data, Destination::sram, "xc7a100t"); });
              validateImage(Bytes{0xff, 0xaa, 0x99, 0x55, 0x66}, Destination::flash, "xc7a100t");
              rejects([] { validateImage(Bytes{0}, Destination::flash, "xc7a100t"); });
              CHECK(firmwareExtension(pathFromText("firmware.BIT")));
          }},
         {"page-aligned memory reads",
          [] {
              std::vector<std::uint8_t> source(16384);
              for (std::size_t i = 0; i < source.size(); ++i)
                  source[i] = static_cast<std::uint8_t>((i * 31 + i / 4096) % 251);
              int calls = 0;
              const auto read = [&](std::uint64_t address, std::span<std::uint8_t> data) {
                  ++calls;
                  CHECK(address % 4096 == 0 && data.size() % 4096 == 0);
                  CHECK(address + data.size() <= source.size());
                  std::copy_n(source.begin() + address, data.size(), data.begin());
                  return true;
              };
              for (const auto address : {0ULL, 8ULL, 4088ULL, 4092ULL, 4096ULL}) {
                  for (const auto size : {8, 16, 4096, 8192}) {
                      std::vector<std::uint8_t> data(static_cast<std::size_t>(size));
                      CHECK(readPageAligned(address, data, read));
                      CHECK(std::equal(data.begin(), data.end(), source.begin() + address));
                  }
              }
              std::vector<std::uint8_t> data(8192);
              calls = 0;
              CHECK(readPageAligned(4096, data, read));
              CHECK(calls == 1);
              source[4096] ^= 0xff;
              CHECK(readPageAligned(4096, std::span(data).first(8), read));
              CHECK(data[0] == source[4096]);
              calls = 0;
              CHECK(readPageAligned(0, {}, read));
              CHECK(!readPageAligned(UINT64_MAX - 3, std::span(data).first(8), read));
              CHECK(calls == 0);
              CHECK(
                  !readPageAligned(4092, std::span(data).first(8), [&](auto address, auto output) {
                      if (address == 4096)
                          return false;
                      return read(address, output);
                  }));
              CHECK(calls == 1);
          }},
         {"benchmark range and overflow",
          [] {
              Benchmark b;
              b.validate();
              int probes = 0;
              CHECK(findReadAddress([&](std::uint64_t address) {
                        ++probes;
                        return address == 3 * automaticReadStride;
                    }) == 3 * automaticReadStride);
              CHECK(probes == 3);
              probes = 0;
              rejects([&] {
                  findReadAddress([&](std::uint64_t) {
                      ++probes;
                      return false;
                  });
              });
              CHECK(probes == 16);
              probes = 0;
              rejects([&] {
                  findReadAddress([&](std::uint64_t) -> bool {
                      ++probes;
                      throw std::runtime_error("cancelled");
                  });
              });
              CHECK(probes == 1);
              b.mode = TransferMode::write;
              b.duration = 3;
              rejects([&] { b.validate(); });
              b.mode = TransferMode::read;
              b.duration = 0;
              b.address = 0x100000000ULL;
              b.dedicated = true;
              b.validate();
              ReadAddresses addresses(b, 8, 42);
              std::set<std::uint64_t> sampled;
              for (int i = 0; i < 1000; ++i) {
                  const auto address = addresses.next();
                  CHECK(address >= b.address && address + 8 <= b.address + b.length);
                  CHECK(address % 8 == 0);
                  sampled.insert(address);
              }
              CHECK(sampled.size() > 900);
              b.randomRead = false;
              ReadAddresses fixed(b, 8, 42);
              for (int i = 0; i < 10; ++i)
                  CHECK(fixed.next() == b.address);
              b.randomRead = true;
              CHECK(b.duration == 0 && b.sizes == std::vector<int>{8} &&
                    b.mode == TransferMode::read);
              Request continuous{BenchmarkRequest{b}, std::chrono::milliseconds(0)};
              continuous.validate();
              CHECK(continuous.deadline() == std::chrono::steady_clock::time_point::max());
              const auto decoded = decodeRequest(encodeRequest(continuous));
              CHECK(decoded.timeout.count() == 0);
              CHECK(std::get<BenchmarkRequest>(decoded.operation).settings.randomRead);
              auto fixedRequest = continuous;
              std::get<BenchmarkRequest>(fixedRequest.operation).settings.randomRead = false;
              CHECK(
                  !std::get<BenchmarkRequest>(decodeRequest(encodeRequest(fixedRequest)).operation)
                       .settings.randomRead);
              CHECK(std::get<BenchmarkRequest>(decoded.operation).settings.duration == 0);
              Request inventory{InventoryRequest{Inventory::system}, std::chrono::milliseconds(0)};
              rejects([&] { inventory.validate(); });
              b.mode = TransferMode::write;
              rejects([&] { b.validate(); });
              b.mode = TransferMode::read;
              b.sizes = {8, 4096};
              rejects([&] { b.validate(); });
              b.sizes = {8};
              b.dedicated = false;
              b.validate();
              b.dedicated = true;
              b.address = 1;
              rejects([&] { b.validate(); });
              b.address = 0xfffffffffffff000ULL;
              b.length = 8192;
              rejects([&] { b.validate(); });
              b.address = 4096;
              b.length = 4096;
              b.sizes = {8192};
              rejects([&] { b.validate(); });
              b.sizes = {4096};
              b.duration = 61;
              rejects([&] { b.validate(); });
          }},
         {"configuration and sensor parsing",
          [] {
              CHECK(parseRegister("idcode 0x03631093\nregister raw value: 0x00004010") == 0x4010U);
              rejects([] { parseRegister("idcode 0x03631093"); });
              auto rows = configurationRows({{"STAT", 0x4010}, {"WBSTAR", 0x20000040}});
              CHECK(std::find(rows.begin(), rows.end(), Row{"Configuration", "Configured"}) !=
                    rows.end());
              auto sensors = sensorRows(
                  R"({"temp":35.5,"mintemp":30,"maxtemp":45,"vccint":1.0,"minvccint":0.98,"maxvccint":1.02,"vccaux":1.8,"minvccaux":1.7,"maxvccaux":1.9,"raw":{"6":21840,},})");
              CHECK(sensors.size() == 4);
              rejects([] { sensorRows("{}"); });
              CHECK(flashRows("Jedec ID: 0xef4018\nManufacturer: Winbond\nCapacity: 128 Mbit")
                        .size() == 3);
          }},
         {"typed request round trip and invalid inputs",
          [] {
              Request request{
                  ProgramRequest{{Backend::ftdi, -1, "FT1234", {}},
                                 {pathFromText("firmware.bit"), Destination::sram, "xc7a100t", 2},
                                 0x0011223344556677ULL},
                  std::chrono::seconds(60)};
              auto decoded = decodeRequest(encodeRequest(request));
              const auto& program = std::get<ProgramRequest>(decoded.operation);
              CHECK(program.firmware.index == 2);
              CHECK(program.connection.backend == Backend::ftdi &&
                    program.connection.serial == "FT1234");
              rejects([] { decodeRequest(R"({"action":"scan","connection":{"backend":3}})"); });
              CHECK(program.expectedDna == 0x0011223344556677ULL);
              CHECK(decoded.programming());
              CHECK(!decoded.privileged());
              rejects([] { decodeRequest(R"({"action":"unknown"})"); });
              rejects([] { decodeRequest(R"({"action":"inventory","kind":99})"); });
              rejects([] {
                  decodeRequest(R"({"action":"scan","connection":{"backend":0},"index":16})");
              });
              rejects([] { decodeRequest(R"({"action":"inventory","kind":0,"timeout_ms":0})"); });
              rejects([] { decodeRequest(std::string(65537, ' ')); });
              Request removal{RemoveHistoryRequest{{"USB\\ONE", "USB\\TWO"}}};
              const auto batch = decodeRequest(encodeRequest(removal));
              CHECK(batch.privileged());
              CHECK(std::get<RemoveHistoryRequest>(batch.operation).instances ==
                    std::get<RemoveHistoryRequest>(removal.operation).instances);
              CHECK(
                  std::get<RemoveHistoryRequest>(
                      decodeRequest(R"({"action":"remove-history","instance":"legacy"})").operation)
                      .instances == std::vector<std::string>{"legacy"});
              rejects([] { Request{RemoveHistoryRequest{{}}}.validate(); });
              rejects([] { Request{RemoveHistoryRequest{{"ONE", "one"}}}.validate(); });
              rejects([] { Request{RemoveHistoryRequest{{"valid", ""}}}.validate(); });
              rejects(
                  [] { Request{RemoveHistoryRequest{{std::string("bad\0id", 6)}}}.validate(); });
          }},
         {"result precision and worker events",
          [] {
              Device d{0,
                       "xc7a100t",
                       "CH347",
                       "ch347",
                       0x03631093,
                       0xfffffffffffffff0ULL,
                       0x0011223344556677ULL,
                       {}};
              Result result = ScanResult{{d}};
              auto decoded = decodeResult(encodeResult(result));
              CHECK(std::get<ScanResult>(decoded).devices[0].fuse == d.fuse);
              auto event = decodeEvent(encodeEvent({{}, {}, result}));
              CHECK(event.result.has_value());
              CHECK(decodeEvent(encodeEvent({{}, "test failure", std::nullopt})).error ==
                    "test failure");
              rejects([] { decodeEvent(R"({"type":"unknown"})"); });
          }},
         {"strict numeric conversion and UTF-8 paths", [] {
              CHECK(number("0xFFFFFFFFFFFFFFFF") == UINT64_MAX);
              rejects([] { number("12junk"); });
              rejects([] { number("-1"); });
              rejects([] { number("18446744073709551616"); });
              CHECK(pathText(pathFromText("C:/test/şöğü.bin")) == "C:/test/şöğü.bin");
          }}});
}
