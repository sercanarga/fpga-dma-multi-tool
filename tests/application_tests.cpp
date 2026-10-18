#include "application/event_stream.hpp"
#include "application/history.hpp"
#include "application/limits.hpp"
#include "core/files.hpp"
#include "test.hpp"
#include <fstream>
#include <random>
namespace {
std::string line(const fpga::Event& event) {
    return fpga::encodeEvent(event) + '\n';
}
}
int main() {
    using namespace fpga;
    using test::rejects;
    return test::run(
        {{"Fragmented worker events preserve output and completion",
          [] {
              std::string output;
              EventStream stream([&](auto value) { output += value; });
              const auto encoded =
                  line({"device şöğü\n", {}, {}}) + line({{}, {}, Notice{"complete"}});
              for (const auto value : encoded)
                  stream.append(std::string(1, value));
              const auto result = stream.finish(0);
              CHECK(output == "device şöğü\n");
              CHECK(result.error.empty());
              CHECK(std::get<Notice>(*result.result).message == "complete");
          }},
         {"Worker completion is terminal and mandatory",
          [] {
              const auto done = line({{}, {}, Notice{"complete"}});
              for (const auto& trailing : {done, line({"late", {}, {}}), line({{}, "late", {}})}) {
                  EventStream stream({});
                  stream.append(done);
                  rejects([&] { stream.append(trailing); });
              }
              EventStream missing({});
              rejects([&] { missing.finish(0); });
              EventStream truncated({});
              truncated.append("{\"type\":");
              rejects([&] { truncated.finish(1); });
              EventStream failedExit({});
              failedExit.append(done);
              rejects([&] { failedExit.finish(1); });
              EventStream error({});
              error.append(line({{}, "Full technical error\nDetails", {}}));
              CHECK(error.finish(7).error == "Full technical error\nDetails");
              rejects([] { decodeEvent(R"({"type":"error","message":""})"); });
          }},
         {"Worker limits apply to individual events",
          [] {
              std::size_t received = 0;
              EventStream stream([&](auto value) { received += value.size(); });
              const auto event = line({std::string(limits::eventBytes / 2, 'x'), {}, {}});
              stream.append(event + event + line({{}, {}, Notice{"done"}}));
              CHECK(stream.finish(0).result.has_value());
              CHECK(received == limits::eventBytes);
              EventStream oversized({});
              rejects([&] { oversized.append(std::string(limits::eventBytes + 1, 'x')); });
          }},
         {"History selection respects visibility and presence",
          [] {
              HistorySelection selection;
              selection.reset({{"USB\\A", "Alpha", "USB", "USB", false},
                               {"USB\\B", "Beta", "USB", "USB", true},
                               {"USB\\C", "Gamma", "USB", "USB", false}});
              selection.filter("", false);
              selection.check({2, 1, 2, 99});
              CHECK(selection.checked() == std::vector<std::size_t>{2});
              CHECK(!selection.allChecked());
              selection.toggleAll();
              CHECK(selection.allChecked());
              CHECK(selection.removal().instances ==
                    std::vector<std::string>({"USB\\A", "USB\\C"}));
              selection.toggleAll();
              rejects([&] { selection.removal(); });
              selection.filter("ALPHA", true);
              CHECK(selection.visible().size() == 1);
              selection.toggleAll();
              CHECK(selection.removal().instances == std::vector<std::string>{"USB\\A"});
              selection.filter("", true);
              CHECK(selection.visible().size() == 2 && selection.checked().empty());
              selection.reset({});
              CHECK(selection.visible().empty() && selection.checked().empty());
          }},
         {"Removal preflight rejects connected, missing and conflicting identities",
          [] {
              std::vector<HistoryEntry> devices{{"USB\\A", "A", "", "", false},
                                                {"USB\\B", "B", "", "", true}};
              validateHistoryRemoval({{"usb\\a"}}, devices);
              rejects([&] { validateHistoryRemoval({{"USB\\B"}}, devices); });
              rejects([&] { validateHistoryRemoval({{"USB\\missing"}}, devices); });
              rejects([&] { validateHistoryRemoval({{"USB\\A", "usb\\a"}}, devices); });
              devices.push_back({"usb\\a", "Connected duplicate", "", "", true});
              rejects([&] { validateHistoryRemoval({{"USB\\A"}}, devices); });
          }},
         {"Bulk removal retains partial failure and restart information",
          [] {
              std::vector<std::string> called;
              const RemoveHistoryRequest request{{"a", "b", "c"}};
              auto report = removeHistoryInstances(
                  request,
                  [&](const auto& id) {
                      called.push_back(id);
                      if (id == "b")
                          throw std::runtime_error("denied");
                      return id == "a";
                  },
                  [] {});
              CHECK(called == request.instances);
              CHECK(report.removed == std::vector<std::string>({"a", "c"}));
              CHECK(report.failures == std::vector<std::string>{"b: denied"});
              CHECK(report.rebootRequired);
              rejects([&] { report.requireSuccess(3); });
              int checks = 0;
              report = removeHistoryInstances(
                  request, [](const auto&) { return false; },
                  [&] {
                      if (++checks == 2)
                          throw std::runtime_error("Operation cancelled");
                  });
              CHECK(report.removed == std::vector<std::string>{"a"});
              CHECK(report.failures == std::vector<std::string>{"Operation cancelled"});
              report = removeHistoryInstances(request, [](const auto&) { return false; }, [] {});
              report.requireSuccess(3);
              CHECK(report.removed.size() == 3);
          }},
         {"Request validation covers direct callers and serialized size",
          [] {
              rejects([] { Request{InventoryRequest{static_cast<Inventory>(99)}}.validate(); });
              rejects(
                  [] { Request{DriverRequest{static_cast<Driver>(99), false, {}}}.validate(); });
              rejects([] {
                  Request{InspectRequest{{}, 0, static_cast<Inspection>(99), {}}}.validate();
              });
              rejects([] {
                  Connection{Backend::automatic, -1, std::string("a\0b", 3), {}}.validate();
              });
              rejects([] {
                  Request{ProgramRequest{{}, {"image.bin", static_cast<Destination>(99)}, {}}}
                      .validate();
              });
              Request read{InventoryRequest{Inventory::system}};
              CHECK(read.inventory() && !read.privileged());
              CHECK(!Request{BenchmarkRequest{}}.inventory());
              RemoveHistoryRequest large;
              for (int i = 0; i < 12; ++i)
                  large.instances.push_back(std::to_string(i) + std::string(3900, '\\'));
              Request{large}.validate();
              rejects([&] { encodeRequest(Request{large}); });
          }},
         {"Bounded file reads preserve binary data and reject oversized inputs", [] {
              const auto path = std::filesystem::temp_directory_path() /
                                ("fpga-file-test-" + std::to_string(std::random_device{}()));
              struct Cleanup {
                  std::filesystem::path path;
                  ~Cleanup() {
                      std::error_code error;
                      std::filesystem::remove(path, error);
                  }
              } cleanup{path};
              {
                  std::ofstream file(path, std::ios::binary);
                  file.write("a\0b", 3);
              }
              CHECK(readFile(path, 3) == std::string("a\0b", 3));
              rejects([&] { readFile(path, 2); });
              {
                  std::ofstream file(path, std::ios::binary | std::ios::trunc);
              }
              CHECK(readFile(path, 0).empty());
              std::filesystem::remove(path);
              rejects([&] { readFile(path, 3); });
          }}});
}
