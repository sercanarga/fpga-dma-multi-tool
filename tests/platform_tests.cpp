#include "application/protocol.hpp"
#include "application/worker.hpp"
#include "core/text.hpp"
#include "platform/devices.hpp"
#include "platform/drivers.hpp"
#include "platform/leechcore_api.hpp"
#include "platform/registry.hpp"
#include "platform/socket.hpp"
#include "test.hpp"
#include "ui/window.hpp"
#include <fstream>
#include <set>
#include <shellapi.h>
#include <shlobj.h>
#include <thread>
namespace {
struct Echo final : fpga::Transport {
    fpga::Bits shift(const fpga::Bits& tms, const fpga::Bits& tdi) override {
        auto result = tdi;
        for (std::size_t i = 0; i < result.size(); ++i)
            result[i] ^= tms[i];
        return result;
    }
};
fpga::Context context(int milliseconds = 5000) {
    return {std::chrono::steady_clock::now() + std::chrono::milliseconds(milliseconds), {}, {}};
}
class WorkerGate {
    fpga::win::Handle event_;

  public:
    WorkerGate()
        : event_(CreateEventW(
              nullptr, TRUE, FALSE,
              (L"Local\\FpgaDmaUiTest-" + std::to_wstring(GetCurrentProcessId())).c_str())) {
        CHECK(event_);
        CHECK(SetEnvironmentVariableW(
            L"FPGA_UI_TEST_GATE",
            (L"Local\\FpgaDmaUiTest-" + std::to_wstring(GetCurrentProcessId())).c_str()));
    }
    ~WorkerGate() {
        SetEvent(event_.get());
        SetEnvironmentVariableW(L"FPGA_UI_TEST_GATE", nullptr);
    }
    void release() {
        CHECK(SetEvent(event_.get()));
    }
};
void pump() {
    MSG message{};
    while (PeekMessageW(&message, nullptr, 0, 0, PM_REMOVE)) {
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
}
}
int main(int argc, char** argv) {
    using namespace fpga;
    if (argc > 1) {
        const std::string mode = argv[1];
        if (mode == "--package-check")
            return test::run(
                {{"Native publisher verification",
                  [] {
                      win::verifySignature(win::runtimePath("drivers/wch/CH341WDM.CAT"),
                                           L"Microsoft Windows Hardware Compatibility Publisher");
                      win::verifySignature(win::runtimePath("drivers/ftdi/ftdibus3.cat"),
                                           L"Microsoft Windows Hardware Compatibility Publisher");
                      win::verifySignature(win::runtimePath("drivers/rs232/zadig-2.9.exe"),
                                           L"Akeo Consulting");
                      test::rejects([] {
                          win::verifySignature(win::runtimePath("drivers/rs232/zadig-2.9.exe"),
                                               L"Wrong publisher");
                      });
                      test::rejects(
                          [] { win::verifySignature(win::executablePath(), L"Akeo Consulting"); });
                  }},
                 {"Runtime DLL loading without hardware access",
                  [] {
                      win::Module ch347(win::runtimePath("drivers/wch/CH347DLLA64.DLL"));
                      CHECK(ch347.function<FARPROC>("CH347OpenDevice"));
                      win::Module leechcore(win::runtimePath("leechcore.dll"));
                      CHECK(leechcore.function<FARPROC>("LcCreateEx"));
                      CHECK(leechcore.function<FARPROC>("LcMemFree"));
                      win::Module ftdi(win::runtimePath("FTD3XX.dll"));
                      CHECK(ftdi.function<FARPROC>("FT_Create"));
                      CHECK(!std::filesystem::exists(win::executablePath().parent_path() /
                                                     L"scripts"));
                  }},
                 {"Small and unaligned LeechCore file reads",
                  [] {
                      win::TemporaryDirectory directory;
                      const auto path = directory.path() / L"memory.bin";
                      std::vector<std::uint8_t> expected(16 * 1024 * 1024);
                      for (std::size_t i = 0; i < expected.size(); ++i)
                          expected[i] = static_cast<std::uint8_t>((i * 31 + i / 4096) % 251);
                      {
                          std::ofstream file(path, std::ios::binary);
                          file.write(reinterpret_cast<const char*>(expected.data()),
                                     static_cast<std::streamsize>(expected.size()));
                          CHECK(file.good());
                      }
                      win::Module library(win::runtimePath("leechcore.dll"));
                      const auto create = library.function<decltype(&LcCreate)>("LcCreate");
                      const auto close = library.function<decltype(&LcClose)>("LcClose");
                      const auto read = library.function<decltype(&LcRead)>("LcRead");
                      LC_CONFIG config{};
                      config.dwVersion = LC_CONFIG_VERSION;
                      const auto name = win::utf8(path.native());
                      CHECK(name.size() < sizeof(config.szDevice));
                      std::copy(name.begin(), name.end(), config.szDevice);
                      const std::unique_ptr<void, decltype(close)> handle(create(&config), close);
                      CHECK(handle);
                      for (const auto address : {0ULL, 8ULL, 4092ULL, 4096ULL, 16773120ULL}) {
                          for (const auto size : {8, 16, 4096}) {
                              std::vector<std::uint8_t> data(static_cast<std::size_t>(size));
                              CHECK(readPageAligned(address, data, [&](auto base, auto output) {
                                  return read(handle.get(), base, static_cast<DWORD>(output.size()),
                                              output.data());
                              }));
                              CHECK(
                                  std::equal(data.begin(), data.end(), expected.begin() + address));
                          }
                      }
                  }},
                 {"Loader dependencies", [] {
                      for (const auto* name : {"openFPGALoader/openFPGALoader.exe",
                                               "openFPGALoader/openFPGALoader-ftdi.exe"}) {
                          auto ctx = context();
                          ProcessOptions options;
                          options.executable = win::runtimePath(name);
                          options.arguments = {L"--help"};
                          CHECK(!execute(ctx, options).output.empty());
                      }
                  }}});
        if (mode == "--worker") {
            wchar_t name[128]{};
            if (GetEnvironmentVariableW(L"FPGA_UI_TEST_GATE", name, 128)) {
                win::Handle gate(OpenEventW(SYNCHRONIZE, FALSE, name));
                if (!gate || WaitForSingleObject(gate.get(), 10000) != WAIT_OBJECT_0)
                    return 1;
            }
            return runWorker();
        }
        if (mode == "--diagnostics") {
            std::cout << "stdout begin\n" << std::string(80000, 'x') << "\nstdout end\n";
            std::cerr << "stderr detail\n";
            return 7;
        }
        if (mode == "--sleep") {
            Sleep(10000);
            return 0;
        }
        if (mode == "--flood") {
            const std::string chunk(8192, 'x');
            for (;;)
                std::cout.write(chunk.data(), static_cast<std::streamsize>(chunk.size()));
        }
        if (mode == "--echo") {
            int count{};
            auto* values = CommandLineToArgvW(GetCommandLineW(), &count);
            for (int i = 2; i < count; ++i)
                std::cout << win::utf8(values[i]) << '\n';
            LocalFree(values);
            return 0;
        }
    }
    const auto initialized = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    const int result = test::run(
        {{"FTDI identity matching",
          [] {
              CHECK(writerPid("USB\\VID_0403&PID_6010&MI_00\\ABC") == "6010");
              CHECK(writerPid("usb\\vid_0403&pid_6014\\ABC") == "6014");
              CHECK(!writerPid("USB\\VID_0403&PID_6010&MI_01\\ABC"));
              CHECK(!writerPid("USB\\VID_0403&PID_6010&MI_000\\ABC"));
              CHECK(!writerPid("USB\\VID_0403&PID_6010extra\\ABC"));
              CHECK(!writerPid("USB\\VID_0403&PID_6010\\"));
              CHECK(!writerPid("USB\\VID_0403&PID_6015\\ABC"));
          }},
         {"Writer INF removal scope",
          [] {
              win::TemporaryDirectory directory;
              const auto path = directory.path() / L"writer.inf";
              const auto write = [&](const std::string& model) {
                  std::ofstream file(path);
                  file << R"([Version]
Signature="$Windows NT$"
Class=USBDevice
ClassGuid={88bae032-5a81-49f0-bc3d-a4ff138216d6}
Provider="Test"
[Manufacturer]
"Test"=Models,NTamd64
[Models.NTamd64]
)" << model << '\n';
              };
              write(R"("Writer"=Install,USB\VID_0403&PID_6010&MI_00)");
              validateWriterPackage(path);
              write(R"("Writer"=Install,USB\VID_0403&PID_6010&MI_01)");
              test::rejects([&] { validateWriterPackage(path); });
              write(R"("Writer"=Install,USB\VID_0403&PID_6010&MI_00
"Other device"=Install,PCI\VEN_1234&DEV_5678)");
              test::rejects([&] { validateWriterPackage(path); });
              write(R"("Writer"=Install,USB\VID_0403&PID_6010&MI_000)");
              test::rejects([&] { validateWriterPackage(path); });
              write(R"("Writer"=Install)");
              test::rejects([&] { validateWriterPackage(path); });
          }},
         {"Native Windows inventory",
          [] {
              auto ctx = context(45000);
              const auto devices = windowsDevices(ctx, true);
              CHECK(!devices.empty());
              for (const auto& item : devices) {
                  CHECK(item.present);
                  CHECK(!item.instance.empty());
              }
              const auto history = deviceHistory(ctx);
              CHECK(history.devices.size() >= devices.size());
              std::set<std::string> ids;
              for (const auto& item : history.devices)
                  CHECK(ids.insert(item.instance).second);
              const auto drivers = driverStatus(ctx);
              CHECK(drivers.rows.size() == 3);
              const auto system = systemInformation(ctx);
              CHECK(std::any_of(system.rows.begin(), system.rows.end(), [](const auto& row) {
                  return row[0] == "Windows build" && row[1] != "Unavailable" && !row[1].empty();
              }));
              win::RegistryKey registry(HKEY_LOCAL_MACHINE,
                                        L"SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion");
              CHECK(!registry.string(L"CurrentBuildNumber").value_or("").empty());
          }},
         {"Administrator request rejects inventory",
          [] {
              Request request{InventoryRequest{Inventory::system}};
              test::rejects([&] { runAdministrator(win::wide(encodeRequest(request))); });
              win::TemporaryDirectory directory;
              const auto path = directory.path() / L"request.json";
              win::atomicWrite(path, encodeRequest(request));
              test::rejects([&] { runAdministratorFile(path, std::string(64, '0')); });
              CHECK(runAdministratorFile(path, win::sha256(path)) == ERROR_FUNCTION_FAILED);
              std::ifstream response(directory.path() / L"result.json");
              const std::string result((std::istreambuf_iterator<char>(response)), {});
              CHECK(decodeEvent(result).error ==
                    "Operation is not a Windows administration request");
          }},
         {"Windows command argument quoting",
          [] {
              auto ctx = context();
              ProcessOptions options;
              options.executable = win::executablePath();
              options.arguments = {L"--echo", L"value with spaces", L"quote\"slash\\",
                                   L"C:\\folder with space\\", L"şöğü"};
              auto result = execute(ctx, options);
              options.captureOutput = false;
              std::string streamed;
              ctx.log = [&](const std::string& chunk) { streamed += chunk; };
              const auto streaming = execute(ctx, options);
              CHECK(streaming.output.empty() && streamed == result.output);
              std::erase(result.output, '\r');
              CHECK(result.output ==
                    "value with spaces\nquote\"slash\\\nC:\\folder with space\\\nşöğü\n");
          }},
         {"Child process timeout and cancellation",
          [] {
              auto ctx = context(100);
              ProcessOptions options;
              options.executable = win::executablePath();
              options.arguments = {L"--sleep"};
              const auto start = std::chrono::steady_clock::now();
              test::rejects([&] { execute(ctx, options); });
              CHECK(std::chrono::steady_clock::now() - start < std::chrono::seconds(2));
              std::stop_source source;
              ctx = context();
              ctx.stop = source.get_token();
              std::jthread stop([&] {
                  std::this_thread::sleep_for(std::chrono::milliseconds(100));
                  source.request_stop();
              });
              test::rejects([&] { execute(ctx, options); });
          }},
         {"Continuous child output respects cancellation and deadline",
          [] {
              ProcessOptions options;
              options.executable = win::executablePath();
              options.arguments = {L"--flood"};
              options.captureOutput = false;
              for (const bool cancel : {false, true}) {
                  auto ctx = context(cancel ? 5000 : 150);
                  std::stop_source source;
                  ctx.stop = source.get_token();
                  std::jthread stopper;
                  if (cancel)
                      stopper = std::jthread([&] {
                          std::this_thread::sleep_for(std::chrono::milliseconds(150));
                          source.request_stop();
                      });
                  const auto start = std::chrono::steady_clock::now();
                  std::string failure;
                  try {
                      execute(ctx, options);
                  } catch (const std::exception& error) {
                      failure = error.what();
                  }
                  CHECK(failure == (cancel ? "Operation cancelled" : "Operation timed out"));
                  CHECK(std::chrono::steady_clock::now() - start < std::chrono::seconds(2));
              }
          }},
         {"Complete child diagnostics",
          [] {
              auto ctx = context();
              std::string streamed;
              ctx.log = [&](const std::string& chunk) { streamed += chunk; };
              ProcessOptions options;
              options.executable = win::executablePath();
              options.arguments = {L"--diagnostics"};
              std::string failure;
              try {
                  execute(ctx, options);
              } catch (const std::exception& exception) {
                  failure = exception.what();
              }
              CHECK(failure.starts_with("Process failed (7): stdout begin"));
              CHECK(failure.find("stdout end") != std::string::npos);
              CHECK(failure.find("stderr detail") != std::string::npos);
              CHECK(failure.size() > 80000);
              CHECK(streamed.find("stdout begin") == 0);
              CHECK(streamed.find("stderr detail") != std::string::npos);
          }},
         {"Local CH347 loader bridge and shutdown",
          [] {
              Echo echo;
              auto ctx = context();
              Bridge bridge(echo, ctx);
              const auto raw = ::socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
              CHECK(raw != INVALID_SOCKET);
              sockaddr_in address{};
              address.sin_family = AF_INET;
              address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
              address.sin_port = htons(bridge.port());
              if (::connect(raw, reinterpret_cast<sockaddr*>(&address), sizeof(address))) {
                  closesocket(raw);
                  throw std::runtime_error("Cannot connect to local loader bridge");
              }
              Socket client(raw);
              const std::string query = "getinfo:";
              client.send(Bytes(query.begin(), query.end()), ctx);
              std::string info;
              do {
                  info += static_cast<char>(client.receive(1, ctx)[0]);
              } while (info.back() != '\n');
              CHECK(info == "xvcServer_v1.0:2048\n");
              const std::string shift = "shift:";
              auto command = Bytes(shift.begin(), shift.end());
              const auto length = little(16384);
              command.insert(command.end(), length.begin(), length.end());
              command.insert(command.end(), 2048, 0xff);
              command.insert(command.end(), 2048, 0);
              client.send(command, ctx);
              CHECK(client.receive(2048, ctx) == Bytes(2048, 0xff));
              bridge.check();
          }},
         {"Documentation preview masks identities and cannot submit hardware work",
          [] {
              ui::Window app({.refreshOnActivation = false,
                              .persistSettings = false,
                              .operationsEnabled = false,
                              .hideIdentifiers = true});
              const auto window = app.create();
              pump();
              test::rejects([&] { app.submit({ScanRequest{}}); });
              CHECK(app.identifier("12345678") == "********");
              Device device;
              device.part = "xc7a100t";
              device.dna = 0x123456789;
              app.present(ui::PageId::devices, ScanResult{{device}});
              const auto tabs = GetDlgItem(window, 1204);
              const auto page = FindWindowExW(tabs, nullptr, L"FpgaDmaPage", L"FPGA Devices");
              const auto table = GetDlgItem(page, 900);
              wchar_t value[64]{};
              ListView_GetItemText(table, 0, 3, value, 64);
              CHECK(std::wstring(value) == std::wstring(16, L'*'));
              ListView_GetItemText(table, 0, 4, value, 64);
              CHECK(std::wstring(value) == std::wstring(18, L'*'));
              for (std::size_t i = 0; i < ui::pageCount; ++i)
                  app.showPage(static_cast<ui::PageId>(i));
              app.present(ui::PageId::drivers, TableResult{{"Driver", "Installed", "Detail"},
                                                           {{"CH347", "Yes", "Installed"},
                                                            {"FTDI D3XX", "Yes", "Installed"},
                                                            {"FTDI writer", "No", "Not installed"}},
                                                           "Checked"});
              RedrawWindow(window, nullptr, nullptr,
                           RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN | RDW_UPDATENOW);
              pump();
              const auto drivers = FindWindowExW(tabs, nullptr, L"FpgaDmaPage", L"Drivers");
              for (int i = 0; i < 3; ++i) {
                  const auto group = GetDlgItem(drivers, 300 + i);
                  const auto dc = GetDC(group);
                  CHECK(dc);
                  const auto background = GetPixel(dc, ui::scaled(group, 5), ui::scaled(group, 30));
                  int textPixels = 0;
                  for (int y = 0; y < ui::scaled(group, 16); ++y)
                      for (int x = ui::scaled(group, 8); x < ui::scaled(group, 220); ++x)
                          if (GetPixel(dc, x, y) == GetSysColor(COLOR_BTNTEXT))
                              ++textPixels;
                  ReleaseDC(group, dc);
                  CHECK(textPixels > 10);
                  CHECK(background == GetSysColor(COLOR_BTNFACE));
              }
              test::rejects([&] { app.showPage(ui::PageId::count); });
              DestroyWindow(window);
              CHECK(app.window() == nullptr);
          }},
         {"Native controls and firmware drop",
          [] {
              ui::Window app({.refreshOnActivation = false, .persistSettings = false});
              auto window = app.create(SW_SHOWNORMAL);
              pump();
              CHECK(GetMenu(window) == nullptr);
              CHECK(SendMessageW(window, WM_GETICON, ICON_SMALL, 0));
              CHECK(SendMessageW(window, WM_GETICON, ICON_BIG, 0));
              const auto credit = FindWindowExW(window, nullptr, WC_LINK, nullptr);
              CHECK(credit);
              LITEM link{};
              link.mask = LIF_ITEMINDEX | LIF_URL;
              CHECK(SendMessageW(credit, LM_GETITEM, 0, reinterpret_cast<LPARAM>(&link)));
              CHECK(std::wstring(link.szUrl) == L"https://github.com/sercanarga");
              CHECK(GetWindowLongPtrW(credit, GWL_STYLE) & WS_CLIPSIBLINGS);
              RECT creditBounds{};
              GetWindowRect(credit, &creditBounds);
              CHECK(creditBounds.bottom - creditBounds.top >= ui::scaled(window, 12));
              RECT statusBounds{};
              GetWindowRect(FindWindowExW(window, nullptr, STATUSCLASSNAMEW, nullptr),
                            &statusBounds);
              CHECK(statusBounds.right >= creditBounds.right);
              CHECK(statusBounds.top < creditBounds.top);
              RECT firstPart{};
              const auto status = FindWindowExW(window, nullptr, STATUSCLASSNAMEW, nullptr);
              CHECK(SendMessageW(status, SB_GETRECT, 0, reinterpret_cast<LPARAM>(&firstPart)));
              MapWindowPoints(status, nullptr, reinterpret_cast<POINT*>(&firstPart), 2);
              CHECK(firstPart.right < creditBounds.left);
              MapWindowPoints(nullptr, window, reinterpret_cast<POINT*>(&creditBounds), 2);
              CHECK(ChildWindowFromPoint(window, {(creditBounds.left + creditBounds.right) / 2,
                                                  (creditBounds.top + creditBounds.bottom) / 2}) ==
                    credit);
              auto tabs = FindWindowExW(window, nullptr, WC_TABCONTROLW, nullptr);
              CHECK(tabs);
              CHECK(TabCtrl_GetItemCount(tabs) == 6);
              CHECK(GetWindowLongPtrW(tabs, GWL_STYLE) & WS_CLIPCHILDREN);
              CHECK(GetWindowLongPtrW(tabs, GWL_EXSTYLE) & WS_EX_CONTROLPARENT);
              CHECK(!FindWindowExW(window, nullptr, WC_COMBOBOXW, nullptr));
              SetWindowPos(window, nullptr, 0, 0, ui::scaled(window, 820), ui::scaled(window, 590),
                           SWP_NOMOVE | SWP_NOZORDER);

              for (int i = 0; i < 6; ++i) {
                  SendMessageW(window, WM_COMMAND, 1100 + i, 0);
                  const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(90);
                  NMHDR changing{tabs, 1204, TCN_SELCHANGING};
                  const auto busy = [&] {
                      return SendMessageW(window, WM_NOTIFY, 1204,
                                          reinterpret_cast<LPARAM>(&changing)) != 0;
                  };
                  do {
                      pump();
                      if (!busy())
                          break;
                      Sleep(10);
                  } while (std::chrono::steady_clock::now() < deadline);
                  CHECK(!busy());
                  CHECK(IsWindowEnabled(tabs));
                  CHECK(TabCtrl_GetCurSel(tabs) == i);
                  HWND page = nullptr;
                  while ((page = FindWindowExW(tabs, page, L"FpgaDmaPage", nullptr))) {
                      if (!IsWindowVisible(page))
                          continue;
                      if (i == 5)
                          CHECK(!GetDlgItem(page, 913));
                      if (i == 2)
                          SendMessageW(GetDlgItem(page, 107), BM_CLICK, 0, 0);
                      if (i == 0 || i == 3 || i == 4 || i == 5) {
                          const auto heading =
                              FindWindowExW(page, nullptr, L"STATIC", ui::text(page).c_str());
                          const auto action = GetDlgItem(page, 100);
                          CHECK(heading && action);
                          RECT titleBounds{}, actionBounds{};
                          GetWindowRect(heading, &titleBounds);
                          GetWindowRect(action, &actionBounds);
                          CHECK(titleBounds.right < actionBounds.left);
                          CHECK(std::abs(titleBounds.top + titleBounds.bottom - actionBounds.top -
                                         actionBounds.bottom) <= 2);
                      }
                      if (i == 1) {
                          SendMessageW(GetDlgItem(page, 111), BM_CLICK, 0, 0);
                          CHECK(ui::checked(GetDlgItem(page, 111)) &&
                                !ui::checked(GetDlgItem(page, 110)));
                          SendMessageW(GetDlgItem(page, 121), BM_CLICK, 0, 0);
                          CHECK(ui::checked(GetDlgItem(page, 121)) &&
                                !ui::checked(GetDlgItem(page, 120)));
                          CHECK(ui::checked(GetDlgItem(page, 111)));
                          SendMessageW(GetDlgItem(page, 120), BM_CLICK, 0, 0);
                          CHECK(ui::checked(GetDlgItem(page, 120)) &&
                                !ui::checked(GetDlgItem(page, 121)));
                          const auto label = FindWindowExW(page, nullptr, L"STATIC", L"Flash");
                          const auto field = GetDlgItem(page, 102);
                          const auto log = GetDlgItem(page, 913);
                          CHECK(label && field && log);
                          auto dc = GetDC(window);
                          CHECK(dc);
                          LOGBRUSH panel{}, labelBrush{}, fieldBrush{}, logBrush{};
                          GetObjectW(
                              reinterpret_cast<HBRUSH>(GetClassLongPtrW(page, GCLP_HBRBACKGROUND)),
                              sizeof(panel), &panel);
                          const auto readBrush = [&](HWND parent, UINT message, HWND control,
                                                     LOGBRUSH& brush) {
                              const auto handle =
                                  SendMessageW(parent, message, reinterpret_cast<WPARAM>(dc),
                                               reinterpret_cast<LPARAM>(control));
                              GetObjectW(reinterpret_cast<HBRUSH>(handle), sizeof(brush), &brush);
                          };
                          readBrush(page, WM_CTLCOLORSTATIC, label, labelBrush);
                          readBrush(page, WM_CTLCOLOREDIT, field, fieldBrush);
                          readBrush(page, WM_CTLCOLORSTATIC, log, logBrush);
                          ReleaseDC(window, dc);
                          CHECK(panel.lbColor == labelBrush.lbColor);
                          CHECK(fieldBrush.lbColor == GetSysColor(COLOR_WINDOW));
                          CHECK(logBrush.lbColor == fieldBrush.lbColor);
                          SendMessageW(window, WM_SYSCOLORCHANGE, 0, 0);
                          pump();
                      }
                      RECT client{};
                      GetClientRect(page, &client);
                      for (HWND child = GetWindow(page, GW_CHILD); child;
                           child = GetWindow(child, GW_HWNDNEXT)) {
                          if (!IsWindowVisible(child))
                              continue;
                          RECT bounds{};
                          GetWindowRect(child, &bounds);
                          MapWindowPoints(nullptr, page, reinterpret_cast<POINT*>(&bounds), 2);
                          CHECK(bounds.left >= 0 && bounds.top >= 0);
                          CHECK(bounds.right <= client.right && bounds.bottom <= client.bottom);
                      }
                  }
              }
              WorkerGate workerGate;
              app.submit({ScanRequest{{Backend::ch347, -1, {}, L"missing-test-library.dll"}, -1},
                          std::chrono::seconds(5)});
              const auto progress = FindWindowExW(status, nullptr, PROGRESS_CLASSW, nullptr);
              CHECK(progress);
              if (!IsWindowVisible(progress)) {
                  for (HWND current = progress; current; current = GetParent(current)) {
                      wchar_t type[64]{};
                      GetClassNameW(current, type, 64);
                      std::cerr << "Progress ancestor " << win::utf8(type) << " style "
                                << GetWindowLongPtrW(current, GWL_STYLE) << " visible "
                                << IsWindowVisible(current) << '\n';
                  }
              }
              CHECK(IsWindowVisible(progress));
              CHECK(IsWindowEnabled(progress));
              CHECK(GetWindowLongPtrW(progress, GWL_STYLE) & PBS_MARQUEE);
              const auto checkProgressBounds = [&] {
                  RECT part{}, bounds{};
                  CHECK(SendMessageW(status, SB_GETRECT, 1, reinterpret_cast<LPARAM>(&part)));
                  GetWindowRect(progress, &bounds);
                  MapWindowPoints(nullptr, status, reinterpret_cast<POINT*>(&bounds), 2);
                  CHECK(bounds.left > part.left && bounds.right < part.right);
                  CHECK(bounds.top > part.top && bounds.bottom < part.bottom);
                  CHECK(bounds.right > bounds.left && bounds.bottom > bounds.top);
                  RECT linkBounds{};
                  GetWindowRect(credit, &linkBounds);
                  MapWindowPoints(status, nullptr, reinterpret_cast<POINT*>(&bounds), 2);
                  CHECK(bounds.right < linkBounds.left);
              };
              checkProgressBounds();
              SetWindowPos(window, nullptr, 0, 0, ui::scaled(window, 960), ui::scaled(window, 680),
                           SWP_NOMOVE | SWP_NOZORDER);
              checkProgressBounds();
              NMHDR changing{tabs, 1204, TCN_SELCHANGING};
              CHECK(SendMessageW(window, WM_NOTIFY, 1204, reinterpret_cast<LPARAM>(&changing)) ==
                    0);
              SendMessageW(window, WM_COMMAND, 1104, 0);
              CHECK(TabCtrl_GetCurSel(tabs) == 4);
              CHECK(IsWindowVisible(progress));
              workerGate.release();
              const auto system = FindWindowExW(tabs, nullptr, L"FpgaDmaPage", L"System Info");
              const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(8);
              while (!IsWindowEnabled(GetDlgItem(system, 100)) &&
                     std::chrono::steady_clock::now() < deadline) {
                  pump();
                  Sleep(10);
              }
              CHECK(IsWindowEnabled(GetDlgItem(system, 100)));
              CHECK(!IsWindowVisible(progress));
              CHECK(SendMessageW(status, SB_GETPARTS, 0, 0) == 2);
              win::TemporaryDirectory directory;
              auto file = directory.path() / L"firmware.bin";
              std::ofstream(file, std::ios::binary).write("\xaa\x99\x55\x66", 4);
              const auto path = file.native();
              const auto bytes = sizeof(DROPFILES) + (path.size() + 2) * sizeof(wchar_t);
              auto memory = GlobalAlloc(GHND, bytes);
              CHECK(memory);
              auto* drop = static_cast<DROPFILES*>(GlobalLock(memory));
              drop->pFiles = sizeof(DROPFILES);
              drop->fWide = TRUE;
              std::memcpy(reinterpret_cast<char*>(drop) + sizeof(DROPFILES), path.c_str(),
                          (path.size() + 1) * sizeof(wchar_t));
              GlobalUnlock(memory);
              SendMessageW(window, WM_DROPFILES, reinterpret_cast<WPARAM>(memory), 0);
              pump();
              CHECK(TabCtrl_GetCurSel(tabs) == 1);
              DestroyWindow(window);
              pump();
          }},
         {"Direct device actions and driver rows",
          [] {
              struct Host final : ui::PageHost {
                  std::optional<Request> request;
                  std::optional<Device> selected;
                  std::optional<TableResult> detail;
                  std::string message;
                  void status(const std::string& value) noexcept override {
                      message = value;
                  }
                  void cancelOperation() override {}
                  Connection connection() const override {
                      return {};
                  }
                  void submit(Request value) override {
                      request = std::move(value);
                  }
                  void target(const Device& value) override {
                      selected = value;
                  }
                  void details(const TableResult& value) override {
                      detail = value;
                  }
              } host;
              ui::Window app({.refreshOnActivation = false, .persistSettings = false});
              auto owner = app.create(SW_SHOWNORMAL);
              auto devices = ui::devicesPage(host);
              devices->create(owner);
              devices->activate();
              CHECK(host.request);
              host.request.reset();
              devices->activate();
              CHECK(host.request);
              CHECK(std::get<ScanRequest>(host.request->operation).connection.backend ==
                    Backend::automatic);
              Device device;
              device.part = "xc7a100t";
              device.index = 2;
              device.dna = 0x123456789;
              devices->complete(ScanResult{{device}});
              CHECK(!GetDlgItem(devices->window(), 107));
              CHECK(IsWindowEnabled(GetDlgItem(devices->window(), 101)));
              const auto deviceTable = GetDlgItem(devices->window(), 900);
              CHECK(Header_GetItemCount(ListView_GetHeader(deviceTable)) == 7);
              wchar_t fuseText[32]{};
              ListView_GetItemText(deviceTable, 0, 4, fuseText, 32);
              CHECK(std::wstring(fuseText) == win::wide("0x" + hex(device.fuse, 16)));
              NMHDR doubleClick{deviceTable, 900, NM_DBLCLK};
              SendMessageW(devices->window(), WM_NOTIFY, 900,
                           reinterpret_cast<LPARAM>(&doubleClick));
              CHECK(!host.detail);
              SendMessageW(devices->window(), WM_COMMAND, 102, 0);
              CHECK(host.selected && host.selected->index == 2);
              CHECK(!FindWindowExW(devices->window(), nullptr, L"BUTTON", L"&Connection..."));
              auto drivers = ui::driversPage(host);
              drivers->create(owner);
              drivers->activate();
              CHECK(std::get<InventoryRequest>(host.request->operation).kind == Inventory::drivers);
              drivers->complete(TableResult{{"Driver", "Installed", "Details"},
                                            {{"FTDI writer", "No", "Connect writer"},
                                             {"CH347", "Yes", "Installed"},
                                             {"FTDI D3XX", "No", "Not installed"}},
                                            ""});
              CHECK(!FindWindowExW(drivers->window(), nullptr, WC_COMBOBOXW, nullptr));
              CHECK(IsWindowEnabled(GetDlgItem(drivers->window(), 201)));
              CHECK(!IsWindowEnabled(GetDlgItem(drivers->window(), 203)));
              CHECK(!IsWindowEnabled(GetDlgItem(drivers->window(), 205)));
              CHECK(ui::text(GetDlgItem(drivers->window(), 200)) == L"&Reinstall");
              CHECK(ui::text(GetDlgItem(drivers->window(), 202)) == L"&Install");
              drivers->setBusy(true, false);
              for (int i = 0; i < 3; ++i) {
                  const auto group = GetDlgItem(drivers->window(), 300 + i);
                  CHECK(group && IsWindowEnabled(group));
                  CHECK((GetWindowLongPtrW(group, GWL_STYLE) & BS_TYPEMASK) == BS_GROUPBOX);
              }
              drivers->setBusy(false, false);
              const auto labelColor = [&](HWND label) {
                  CHECK(label);
                  const auto dc = GetDC(label);
                  CHECK(dc);
                  SendMessageW(drivers->window(), WM_CTLCOLORSTATIC, reinterpret_cast<WPARAM>(dc),
                               reinterpret_cast<LPARAM>(label));
                  const auto color = GetTextColor(dc);
                  ReleaseDC(label, dc);
                  return color;
              };
              const auto installed =
                  FindWindowExW(drivers->window(), nullptr, L"STATIC", L"Installed");
              const auto missing =
                  FindWindowExW(drivers->window(), nullptr, L"STATIC", L"Not installed");
              CHECK(labelColor(installed) ==
                    ui::statusColor(ui::StatusTone::positive, GetSysColor(COLOR_BTNFACE))
                        .value_or(GetSysColor(COLOR_BTNTEXT)));
              CHECK(labelColor(missing) ==
                    ui::statusColor(ui::StatusTone::negative, GetSysColor(COLOR_BTNFACE))
                        .value_or(GetSysColor(COLOR_BTNTEXT)));
              drivers->complete(TableResult{{"Driver", "Installed", "Details"}, {}, ""});
              CHECK(labelColor(installed) == GetSysColor(COLOR_BTNTEXT));
              auto history = ui::historyPage(host);
              history->create(owner);
              CHECK(ui::checked(GetDlgItem(history->window(), 104)));
              history->complete(HistoryResult{{{"one", "Connected", "USB", "USB", true},
                                               {"two", "Disconnected", "USB", "USB", false}}});
              CHECK(ListView_GetItemCount(GetDlgItem(history->window(), 900)) == 1);
              ui::check(GetDlgItem(history->window(), 104), false);
              SendMessageW(history->window(), WM_COMMAND, MAKEWPARAM(104, BN_CLICKED), 0);
              CHECK(ListView_GetItemCount(GetDlgItem(history->window(), 900)) == 2);
              const auto historyTable = GetDlgItem(history->window(), 900);
              CHECK(!GetDlgItem(history->window(), 105));
              const auto historyHeader = ListView_GetHeader(historyTable);
              CHECK(GetWindowLongPtrW(historyHeader, GWL_STYLE) & HDS_CHECKBOXES);
              const auto headerChecked = [&] {
                  HDITEMW item{};
                  item.mask = HDI_FORMAT;
                  CHECK(Header_GetItem(historyHeader, 0, &item));
                  CHECK(item.fmt & HDF_CHECKBOX);
                  return (item.fmt & HDF_CHECKED) != 0;
              };
              const auto toggleAll = [&] {
                  NMHEADERW click{};
                  click.hdr = {historyHeader, 0, HDN_ITEMSTATEICONCLICK};
                  SendMessageW(history->window(), WM_NOTIFY, 0, reinterpret_cast<LPARAM>(&click));
              };
              CHECK(!headerChecked());
              CHECK(ListView_GetExtendedListViewStyle(historyTable) & LVS_EX_CHECKBOXES);
              CHECK(Header_GetItemCount(historyHeader) == 6);
              CHECK(ListView_GetColumnWidth(historyTable, 0) == ui::scaled(historyTable, 32));
              wchar_t nameColumn[32]{};
              LVCOLUMNW column{};
              column.mask = LVCF_TEXT;
              column.pszText = nameColumn;
              column.cchTextMax = 32;
              CHECK(
                  SendMessageW(historyTable, LVM_GETCOLUMNW, 1, reinterpret_cast<LPARAM>(&column)));
              CHECK(std::wstring(nameColumn) == L"Name");
              ListView_SetCheckState(historyTable, 0, TRUE);
              CHECK(!ListView_GetCheckState(historyTable, 0));
              history->complete(HistoryResult{{{"one", "Connected", "USB", "USB", true},
                                               {"two", "Zulu", "USB", "USB", false},
                                               {"three", "Alpha", "USB", "USB", false}}});
              toggleAll();
              CHECK(!ListView_GetCheckState(historyTable, 0));
              CHECK(ListView_GetCheckState(historyTable, 1));
              CHECK(ListView_GetCheckState(historyTable, 2));
              CHECK(headerChecked());
              CHECK(IsWindowEnabled(GetDlgItem(history->window(), 101)));
              ListView_SetCheckState(historyTable, 2, FALSE);
              CHECK(!headerChecked());
              NMLISTVIEW historySort{};
              historySort.hdr = {historyTable, 900, LVN_COLUMNCLICK};
              historySort.iSubItem = 1;
              SendMessageW(history->window(), WM_NOTIFY, 900,
                           reinterpret_cast<LPARAM>(&historySort));
              CHECK(!ListView_GetCheckState(historyTable, 0));
              CHECK(!ListView_GetCheckState(historyTable, 1));
              CHECK(ListView_GetCheckState(historyTable, 2));
              history->setBusy(true, false);
              toggleAll();
              CHECK(!ListView_GetCheckState(historyTable, 0));
              ListView_SetCheckState(historyTable, 2, FALSE);
              CHECK(ListView_GetCheckState(historyTable, 2));
              CHECK(!IsWindowEnabled(GetDlgItem(history->window(), 101)));
              history->setBusy(false, false);
              ui::text(GetDlgItem(history->window(), 103), L"Alpha");
              CHECK(ListView_GetItemCount(historyTable) == 1);
              CHECK(!ListView_GetCheckState(historyTable, 0));
              toggleAll();
              CHECK(ListView_GetCheckState(historyTable, 0));
              toggleAll();
              CHECK(!ListView_GetCheckState(historyTable, 0));
              CHECK(!IsWindowEnabled(GetDlgItem(history->window(), 101)));
              history->activate();
              CHECK(std::get<InventoryRequest>(host.request->operation).kind == Inventory::history);
              CHECK(!GetDlgItem(history->window(), 913));
              auto system = ui::systemPage(host);
              system->create(owner);
              system->activate();
              CHECK(std::get<InventoryRequest>(host.request->operation).kind == Inventory::system);
              system->complete(TableResult{
                  {"Property", "Value"},
                  {{"Secure Boot", "Yes"}, {"Memory integrity", "No"}, {"TPM", "Unavailable"}},
                  ""});
              const auto systemTable = GetDlgItem(system->window(), 900);
              CHECK(ListView_IsGroupViewEnabled(systemTable));
              CHECK(ListView_GetGroupCount(systemTable) == 2);
              wchar_t groupTitle[64]{};
              LVGROUP group{};
              group.cbSize = sizeof(group);
              group.mask = LVGF_HEADER;
              group.pszHeader = groupTitle;
              group.cchHeader = 64;
              CHECK(ListView_GetGroupInfo(systemTable, 0, &group) != -1);
              CHECK(std::wstring(groupTitle) == L"System and firmware");
              const auto cellColor = [&](int row, int column) {
                  NMLVCUSTOMDRAW draw{};
                  draw.dwItemType = LVCDI_ITEMSLIST;
                  draw.nmcd.hdr = {systemTable, 900, NM_CUSTOMDRAW};
                  draw.nmcd.dwDrawStage = CDDS_PREPAINT;
                  CHECK(SendMessageW(system->window(), WM_NOTIFY, 900,
                                     reinterpret_cast<LPARAM>(&draw)) == CDRF_NOTIFYITEMDRAW);
                  draw.dwItemType = LVCDI_ITEM;
                  draw.nmcd.dwDrawStage = CDDS_ITEMPREPAINT;
                  draw.nmcd.dwItemSpec = static_cast<DWORD_PTR>(row);
                  CHECK(SendMessageW(system->window(), WM_NOTIFY, 900,
                                     reinterpret_cast<LPARAM>(&draw)) == CDRF_NOTIFYSUBITEMDRAW);
                  draw.nmcd.dwDrawStage |= CDDS_SUBITEM;
                  draw.iSubItem = column;
                  draw.clrText = GetSysColor(COLOR_WINDOWTEXT);
                  SendMessageW(system->window(), WM_NOTIFY, 900, reinterpret_cast<LPARAM>(&draw));
                  return draw.clrText;
              };
              const auto positive =
                  ui::statusColor(ui::StatusTone::positive, GetSysColor(COLOR_WINDOW))
                      .value_or(GetSysColor(COLOR_WINDOWTEXT));
              const auto negative =
                  ui::statusColor(ui::StatusTone::negative, GetSysColor(COLOR_WINDOW))
                      .value_or(GetSysColor(COLOR_WINDOWTEXT));
              CHECK(cellColor(0, 1) == GetSysColor(COLOR_WINDOWTEXT));
              CHECK(cellColor(1, 1) == positive);
              CHECK(cellColor(2, 1) == negative);
              CHECK(cellColor(0, 0) == GetSysColor(COLOR_WINDOWTEXT));
              ListView_SetGroupState(systemTable, 0, LVGS_COLLAPSED, LVGS_COLLAPSED);
              NMLISTVIEW sort{};
              sort.hdr = {systemTable, 900, LVN_COLUMNCLICK};
              SendMessageW(system->window(), WM_NOTIFY, 900, reinterpret_cast<LPARAM>(&sort));
              CHECK(ListView_GetGroupState(systemTable, 0, LVGS_COLLAPSED) == LVGS_COLLAPSED);
              CHECK(cellColor(1, 1) == negative);
              CHECK(cellColor(2, 1) == positive);
              ListView_SetItemState(systemTable, 2, LVIS_SELECTED, LVIS_SELECTED);
              CHECK(cellColor(2, 1) == GetSysColor(COLOR_WINDOWTEXT));
              auto flash = ui::firmwarePage(host);
              flash->create(owner);
              SetWindowPos(flash->window(), nullptr, 0, 0, ui::scaled(owner, 780),
                           ui::scaled(owner, 500), SWP_NOZORDER);
              CHECK(
                  !(GetWindowLongPtrW(GetDlgItem(flash->window(), 102), GWL_STYLE) & ES_READONLY));
              CHECK(ui::checked(GetDlgItem(flash->window(), 120)));
              CHECK(ui::checked(GetDlgItem(flash->window(), 110)));
              RECT writerBounds{}, modeBounds{};
              GetWindowRect(GetDlgItem(flash->window(), 112), &writerBounds);
              GetWindowRect(GetDlgItem(flash->window(), 120), &modeBounds);
              CHECK(writerBounds.top == modeBounds.top);
              CHECK(writerBounds.right < modeBounds.left);
              CHECK(!GetDlgItem(flash->window(), 910));
              CHECK(!GetDlgItem(flash->window(), 911));
              CHECK(GetWindowLongPtrW(GetDlgItem(flash->window(), 913), GWL_STYLE) & WS_VISIBLE);
              CHECK(!IsWindowEnabled(GetDlgItem(flash->window(), 101)));
              ui::text(GetDlgItem(flash->window(), 102), L"C:\\firmware.bin");
              CHECK(IsWindowEnabled(GetDlgItem(flash->window(), 101)));
              ui::text(GetDlgItem(flash->window(), 102), L"C:\\firmware.bit");
              CHECK(ui::checked(GetDlgItem(flash->window(), 121)));
              CHECK(!ui::checked(GetDlgItem(flash->window(), 120)));
              flash->target(device);
              const auto identity = win::wide("DNA " + hex(device.dna, 16) + "  /  Chain " +
                                              std::to_string(device.index));
              SendMessageW(flash->window(), WM_COMMAND, MAKEWPARAM(110, BN_CLICKED), 0);
              CHECK(FindWindowExW(flash->window(), nullptr, L"STATIC", identity.c_str()));
              SendMessageW(flash->window(), WM_COMMAND, MAKEWPARAM(111, BN_CLICKED), 0);
              CHECK(!FindWindowExW(flash->window(), nullptr, L"STATIC", identity.c_str()));
              flash->appendLog("flash only\n");
              devices->appendLog("device only\n");
              drivers->appendLog("driver only\n");
              CHECK(ui::text(GetDlgItem(flash->window(), 913)) == L"flash only\r\n");
              CHECK(!GetDlgItem(devices->window(), 913));
              CHECK(!GetDlgItem(drivers->window(), 913));
              CHECK(!GetDlgItem(drivers->window(), 910));
              CHECK(ui::text(GetDlgItem(flash->window(), 913)) == L"flash only\r\n");
              auto speed = ui::benchmarkPage(host);
              speed->create(owner);
              SetWindowPos(speed->window(), nullptr, 0, 0, ui::scaled(owner, 780),
                           ui::scaled(owner, 500), SWP_NOZORDER | SWP_SHOWWINDOW);
              CHECK(!FindWindowExW(speed->window(), nullptr, WC_LISTVIEWW, nullptr));
              const auto outputStyle =
                  GetWindowLongPtrW(GetDlgItem(speed->window(), 109), GWL_STYLE);
              CHECK(outputStyle & ES_AUTOHSCROLL);
              CHECK(outputStyle & WS_HSCROLL);
              RECT startBounds{}, advancedBounds{};
              GetWindowRect(GetDlgItem(speed->window(), 100), &startBounds);
              GetWindowRect(GetDlgItem(speed->window(), 107), &advancedBounds);
              CHECK(startBounds.top == advancedBounds.top);
              CHECK(advancedBounds.right <= startBounds.left);
              CHECK(!GetDlgItem(speed->window(), 910));
              CHECK(!GetDlgItem(speed->window(), 913));
              CHECK(!(GetWindowLongPtrW(GetDlgItem(speed->window(), 103), GWL_STYLE) & WS_VISIBLE));
              host.request.reset();
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(100, BN_CLICKED), 0);
              CHECK(host.request);
              CHECK(std::get<BenchmarkRequest>(host.request->operation).settings.address == 0);
              CHECK(!(GetWindowLongPtrW(GetDlgItem(speed->window(), 103), GWL_STYLE) & WS_VISIBLE));
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(107, BN_CLICKED), 0);
              CHECK(GetWindowLongPtrW(GetDlgItem(speed->window(), 103), GWL_STYLE) & WS_VISIBLE);
              for (int id : {101, 102, 103, 104, 105, 110}) {
                  RECT field{};
                  GetWindowRect(GetDlgItem(speed->window(), id), &field);
                  CHECK(field.right - field.left == ui::scaled(owner, 240));
              }
              CHECK(ui::selection(GetDlgItem(speed->window(), 101)) == 0);
              CHECK(ui::selection(GetDlgItem(speed->window(), 105)) == 0);
              ui::text(GetDlgItem(speed->window(), 103), L"0x100000000");
              ui::check(GetDlgItem(speed->window(), 106), true);
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(100, BN_CLICKED), 0);
              CHECK(host.request->timeout.count() == 0);
              const auto defaults = std::get<BenchmarkRequest>(host.request->operation).settings;
              CHECK(defaults.sizes == std::vector<int>{8} && defaults.duration == 0 &&
                    defaults.randomRead);
              ui::select(GetDlgItem(speed->window(), 101), 2);
              ui::select(GetDlgItem(speed->window(), 105), 4);
              ui::select(GetDlgItem(speed->window(), 102), 1);
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(100, BN_CLICKED), 0);
              const auto benchmark = std::get<BenchmarkRequest>(host.request->operation).settings;
              CHECK(benchmark.mode == TransferMode::both &&
                    benchmark.sizes == std::vector<int>{16777216});
              CHECK(benchmark.address == 0x100000000 && benchmark.dedicated);
              speed->appendLog("Read: 4000 reads/s\n");
              CHECK(ui::text(GetDlgItem(speed->window(), 109)) ==
                    L"Opening DMA device...\r\nRead: 4000 reads/s\r\n");
              SendMessageW(GetDlgItem(speed->window(), 109), EM_SETSEL, 0, 7);
              speed->appendLog("Read: 6500 reads/s\n");
              CHECK(ui::text(GetDlgItem(speed->window(), 109)) ==
                    L"Opening DMA device...\r\nRead: 4000 reads/s\r\nRead: 6500 reads/s\r\n");
              speed->complete(BenchmarkResult{{{TransferMode::read, 4096, 15.625, 4000, 250}}});
              CHECK(ui::text(GetDlgItem(speed->window(), 109)).find(L"Average results") !=
                    std::wstring::npos);
              speed->setBusy(true, true);
              speed->setBusy(true, true);
              CHECK(IsWindowEnabled(GetDlgItem(speed->window(), 109)));
              CHECK(IsWindowEnabled(GetDlgItem(speed->window(), 108)));
              CHECK(!IsWindowEnabled(GetDlgItem(speed->window(), 107)));
              speed->setBusy(false, false);
              CHECK(IsWindowEnabled(GetDlgItem(speed->window(), 100)));
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(107, BN_CLICKED), 0);
              CHECK(!(GetWindowLongPtrW(GetDlgItem(speed->window(), 103), GWL_STYLE) & WS_VISIBLE));
              host.request.reset();
              ui::text(GetDlgItem(speed->window(), 104), L"invalid");
              SendMessageW(speed->window(), WM_COMMAND, MAKEWPARAM(100, BN_CLICKED), 0);
              CHECK(!host.request && !host.message.empty());
              CHECK(ui::text(GetDlgItem(speed->window(), 109)).find(win::wide(host.message)) !=
                    std::wstring::npos);
              const auto diagnostic =
                  "diagnostic begin\n" + std::string(80000, 'x') + "\ndiagnostic end\n";
              ui::text(GetDlgItem(speed->window(), 109), L"");
              speed->appendLog(diagnostic);
              const auto output = ui::text(GetDlgItem(speed->window(), 109));
              CHECK(output.starts_with(L"diagnostic begin\r\n"));
              CHECK(output.ends_with(L"diagnostic end\r\n"));
              CHECK(output.size() > 80000);
              speed->appendLog(std::string(16 * 1024 * 1024, 'x') + "\nfinal diagnostic\n");
              const auto boundedOutput = ui::text(GetDlgItem(speed->window(), 109));
              CHECK(boundedOutput.size() <= 16 * 1024 * 1024);
              CHECK(boundedOutput.starts_with(L"[Earlier output removed: display limit]\r\n"));
              CHECK(boundedOutput.ends_with(L"final diagnostic\r\n"));
              devices->setBusy(true, true);
              CHECK(IsWindowEnabled(GetDlgItem(devices->window(), 900)));
              CHECK(!IsWindowEnabled(GetDlgItem(devices->window(), 100)));
              CHECK(!GetDlgItem(devices->window(), 914));
              const auto title =
                  FindWindowExW(devices->window(), nullptr, L"STATIC", L"FPGA Devices");
              CHECK(IsWindowEnabled(title));
              ListView_SetItemState(GetDlgItem(devices->window(), 900), -1, 0, LVIS_SELECTED);
              devices->setBusy(false, false);
              CHECK(!IsWindowEnabled(GetDlgItem(devices->window(), 101)));
              CHECK(IsWindowEnabled(GetDlgItem(devices->window(), 100)));
              flash->setBusy(true, true);
              CHECK(!IsWindowEnabled(GetDlgItem(flash->window(), 102)));
              CHECK(IsWindowEnabled(GetDlgItem(flash->window(), 913)));
              CHECK(IsWindowEnabled(GetDlgItem(flash->window(), 912)));
              ui::text(GetDlgItem(flash->window(), 102), L"");
              flash->setBusy(false, false);
              CHECK(IsWindowEnabled(GetDlgItem(flash->window(), 102)));
              CHECK(!IsWindowEnabled(GetDlgItem(flash->window(), 101)));
              CHECK((GetWindowLongPtrW(GetDlgItem(flash->window(), 110), GWL_STYLE) &
                     BS_TYPEMASK) == BS_AUTORADIOBUTTON);
              DestroyWindow(owner);
              pump();
          }},
         {"Worker mailbox cancellation and reuse",
          [] {
              auto receiver = CreateWindowExW(0, L"STATIC", L"", 0, 0, 0, 0, 0, HWND_MESSAGE,
                                              nullptr, GetModuleHandleW(nullptr), nullptr);
              CHECK(receiver);
              struct Receiver {
                  HWND window;
                  ~Receiver() {
                      DestroyWindow(window);
                  }
              } owner{receiver};
              WorkQueue queue(receiver);
              Request request{
                  ScanRequest{{Backend::ch347, -1, {}, L"missing-test-library.dll"}, -1},
                  std::chrono::seconds(5)};
              const auto finish = [&] {
                  auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
                  while (std::chrono::steady_clock::now() < deadline) {
                      for (auto& update : queue.take())
                          if (auto* completion = std::get_if<Completion>(&update))
                              return *completion;
                      std::this_thread::sleep_for(std::chrono::milliseconds(10));
                  }
                  throw std::runtime_error("Worker mailbox did not complete");
              };
              {
                  WorkerGate gate;
                  queue.start(request);
                  const auto start = std::chrono::steady_clock::now();
                  test::rejects([&] { queue.start(request); });
                  CHECK(std::chrono::steady_clock::now() - start < std::chrono::seconds(1));
                  queue.cancel();
                  CHECK(finish().error == "Operation cancelled");
              }
              queue.start(request);
              const auto completion = finish();
              CHECK(!completion.error.empty() && completion.error != "Operation cancelled");
              CHECK(!completion.result);
          }},
         {"Atomic report replacement",
          [] {
              win::TemporaryDirectory directory;
              auto file = directory.path() / L"report.json";
              win::atomicWrite(file, "first");
              win::atomicWrite(file, "second");
              std::ifstream stream(file);
              std::string value;
              stream >> value;
              CHECK(value == "second");
          }},
         {"Worker request transport", [] {
              auto ctx = context();
              ProcessOptions options;
              options.executable = win::executablePath();
              options.arguments = {L"--worker"};
              options.input = R"({"action":"unknown"})";
              options.allowFailure = true;
              auto result = execute(ctx, options);
              CHECK(result.exitCode == 1);
              CHECK(result.output.find("Unknown operation") != std::string::npos);
          }}});
    if (SUCCEEDED(initialized))
        CoUninitialize();
    return result;
}
