#include "platform/socket.hpp"
#include "platform/timer.hpp"
#include "test.hpp"
#include <chrono>
#include <thread>
using namespace fpga;
namespace {
Context context() {
    return {std::chrono::steady_clock::now() + std::chrono::seconds(15), {}, {}};
}
Socket connect(std::uint16_t port) {
    const auto raw = ::socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    CHECK(raw != INVALID_SOCKET);
    sockaddr_in address{};
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    address.sin_port = htons(port);
    if (::connect(raw, reinterpret_cast<sockaddr*>(&address), sizeof(address))) {
        closesocket(raw);
        throw std::runtime_error("Cannot connect to test bridge");
    }
    Socket client(raw);
    int noDelay = 0, length = sizeof(noDelay);
    CHECK(!getsockopt(raw, IPPROTO_TCP, TCP_NODELAY, reinterpret_cast<char*>(&noDelay), &length));
    CHECK(noDelay == 1);
    return client;
}
struct Echo final : Transport {
    Bits shift(const Bits& tms, const Bits& tdi) override {
        auto result = tdi;
        for (std::size_t i = 0; i < result.size(); ++i)
            result[i] ^= tms[i];
        return result;
    }
};
Bytes shiftPacket(std::size_t count) {
    Bytes packet{'s', 'h', 'i', 'f', 't', ':'};
    const auto length = little(static_cast<std::uint32_t>(count));
    packet.insert(packet.end(), length.begin(), length.end());
    packet.insert(packet.end(), (count + 7) / 8, 0xff);
    packet.insert(packet.end(), (count + 7) / 8, 0);
    return packet;
}
template <class Wait> double elapsed(Wait wait, int count) {
    const auto start = std::chrono::steady_clock::now();
    for (int i = 0; i < count; ++i)
        wait();
    return std::chrono::duration<double, std::milli>(std::chrono::steady_clock::now() - start)
        .count();
}
}
int main(int argc, char** argv) {
    if (argc == 2 && std::string_view(argv[1]) == "--benchmark") {
        win::ShortWait timer(std::chrono::microseconds(250));
        const auto before =
            elapsed([] { std::this_thread::sleep_for(std::chrono::microseconds(250)); }, 256);
        const auto after = elapsed([&] { timer.wait(); }, 256);
        std::cout << "256 transfer gaps: previous_ms=" << before << " corrected_ms=" << after
                  << " ratio=" << before / after << '\n';
        return 0;
    }
    return test::run({
        {"Reusable timer preserves the 250 microsecond pacing minimum",
         [] {
             win::ShortWait timer(std::chrono::microseconds(250));
             CHECK(elapsed([&] { timer.wait(); }, 128) >= 32.0);
             test::rejects([] { win::ShortWait timer(std::chrono::microseconds(0)); });
             test::rejects([] { win::ShortWait timer(std::chrono::seconds(2)); });
         }},
        {"Buffered socket preserves bytes across moves and enforces cancellation",
         [] {
             auto ctx = context();
             std::uint16_t port = 0;
             auto listener = Socket::listen(port);
             auto client = connect(port);
             auto server = listener.accept(ctx);
             Bytes payload(32768);
             for (std::size_t i = 0; i < payload.size(); ++i)
                 payload[i] = static_cast<std::uint8_t>(i * 13);
             server.send(payload, ctx);
             CHECK(client.receive(1, ctx) == Bytes{payload[0]});
             Socket moved(std::move(client));
             client = std::move(moved);
             CHECK(client.receive(payload.size() - 1, ctx) ==
                   Bytes(payload.begin() + 1, payload.end()));
             test::rejects([&] { client.receive(32769, ctx); });
             server.send(Bytes{1, 2}, ctx);
             CHECK(client.receive(1, ctx) == Bytes{1});
             std::stop_source stop;
             stop.request_stop();
             ctx.stop = stop.get_token();
             test::rejects([&] { client.receive(1, ctx); });
         }},
        {"XVC coalesced and fragmented packets retain framing and bit order",
         [] {
             Echo echo;
             auto ctx = context();
             Bridge bridge(echo, ctx);
             auto client = connect(bridge.port());
             Bytes query{'g', 'e', 't', 'i', 'n', 'f', 'o', ':'};
             const Bytes clock{'s', 'e', 't', 't', 'c', 'k', ':', 1, 0, 0, 0};
             query.insert(query.end(), clock.begin(), clock.end());
             const auto shortShift = shiftPacket(9);
             query.insert(query.end(), shortShift.begin(), shortShift.end());
             client.send(query, ctx);
             const std::string info = "xvcServer_v1.0:2048\n";
             CHECK(client.receive(info.size(), ctx) == Bytes(info.begin(), info.end()));
             CHECK(client.receive(4, ctx) == little(2133));
             CHECK(client.receive(2, ctx) == Bytes({0xff, 1}));
             const auto packet = shiftPacket(16384);
             for (std::size_t offset = 0; offset < packet.size();) {
                 const auto length = std::min<std::size_t>(73, packet.size() - offset);
                 client.send(std::span(packet).subspan(offset, length), ctx);
                 offset += length;
             }
             CHECK(client.receive(2048, ctx) == Bytes(2048, 0xff));
             for (int i = 0; i < 128; ++i) {
                 client.send(shortShift, ctx);
                 CHECK(client.receive(2, ctx) == Bytes({0xff, 1}));
             }
             bridge.check();
         }},
    });
}
