#include "platform/socket.hpp"
#include "core/text.hpp"
#include <algorithm>
#include <memory>
#include <thread>
namespace fpga {
namespace {
class ConnectionClosed final : public std::runtime_error {
  public:
    ConnectionClosed() : std::runtime_error("XVC connection closed") {}
};
void initialize() {
    static const auto initialized = [] {
        WSADATA data{};
        if (WSAStartup(MAKEWORD(2, 2), &data))
            fail("Cannot initialize Winsock");
        return true;
    }();
    static_cast<void>(initialized);
}
Bytes bytes(std::string_view text) {
    return {text.begin(), text.end()};
}
}
Socket::Socket(SOCKET value) : value_(value) {
    if (value_ == INVALID_SOCKET)
        fail("Cannot create socket");
    u_long enabled = 1;
    const int noDelay = 1;
    if (ioctlsocket(value_, FIONBIO, &enabled) ||
        setsockopt(value_, IPPROTO_TCP, TCP_NODELAY, reinterpret_cast<const char*>(&noDelay),
                   sizeof(noDelay))) {
        closesocket(value_);
        value_ = INVALID_SOCKET;
        fail("Cannot configure socket");
    }
}
Socket::~Socket() {
    if (value_ != INVALID_SOCKET)
        closesocket(value_);
}
Socket::Socket(Socket&& other) noexcept
    : value_(std::exchange(other.value_, INVALID_SOCKET)), buffered_(std::move(other.buffered_)),
      consumed_(std::exchange(other.consumed_, 0)) {}
Socket& Socket::operator=(Socket&& other) noexcept {
    if (this != &other) {
        if (value_ != INVALID_SOCKET)
            closesocket(value_);
        value_ = std::exchange(other.value_, INVALID_SOCKET);
        buffered_ = std::move(other.buffered_);
        consumed_ = std::exchange(other.consumed_, 0);
    }
    return *this;
}
void Socket::wait(bool writing, Context& context) const {
    for (;;) {
        context.check();
        fd_set ready;
        FD_ZERO(&ready);
        FD_SET(value_, &ready);
        timeval timeout{0, 50000};
        const int count =
            select(0, writing ? nullptr : &ready, writing ? &ready : nullptr, nullptr, &timeout);
        if (count > 0)
            return;
        if (count == SOCKET_ERROR)
            fail("Socket wait failed");
    }
}
Socket Socket::listen(std::uint16_t& port) {
    initialize();
    Socket socket(::socket(AF_INET, SOCK_STREAM, IPPROTO_TCP));
    sockaddr_in address{};
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    if (bind(socket.value_, reinterpret_cast<sockaddr*>(&address), sizeof(address)) ||
        ::listen(socket.value_, 1))
        fail("Cannot start local XVC bridge");
    int size = sizeof(address);
    if (getsockname(socket.value_, reinterpret_cast<sockaddr*>(&address), &size))
        fail("Cannot read bridge port");
    port = ntohs(address.sin_port);
    return socket;
}
Socket Socket::accept(Context& context) const {
    for (;;) {
        wait(false, context);
        auto socket = ::accept(value_, nullptr, nullptr);
        if (socket != INVALID_SOCKET)
            return Socket(socket);
        if (WSAGetLastError() != WSAEWOULDBLOCK)
            fail("XVC accept failed");
    }
}
void Socket::send(std::span<const std::uint8_t> data, Context& context) const {
    while (!data.empty()) {
        wait(true, context);
        const auto count = ::send(value_, reinterpret_cast<const char*>(data.data()),
                                  static_cast<int>(std::min<std::size_t>(data.size(), 16384)), 0);
        if (count > 0)
            data = data.subspan(static_cast<std::size_t>(count));
        else if (count == 0 || WSAGetLastError() != WSAEWOULDBLOCK)
            throw ConnectionClosed{};
    }
}
Bytes Socket::receive(std::size_t count, Context& context) {
    if (count > 32768)
        fail("XVC packet exceeds limit");
    Bytes data(count);
    std::size_t offset = 0;
    while (offset < count) {
        context.check();
        if (consumed_ == buffered_.size()) {
            wait(false, context);
            buffered_.resize(16384);
            const auto received = recv(value_, reinterpret_cast<char*>(buffered_.data()),
                                       static_cast<int>(buffered_.size()), 0);
            consumed_ = 0;
            if (received <= 0) {
                buffered_.clear();
                if (received == 0 || WSAGetLastError() != WSAEWOULDBLOCK)
                    throw ConnectionClosed{};
                continue;
            }
            buffered_.resize(static_cast<std::size_t>(received));
        }
        const auto available = std::min(count - offset, buffered_.size() - consumed_);
        std::copy_n(buffered_.begin() + static_cast<std::ptrdiff_t>(consumed_), available,
                    data.begin() + static_cast<std::ptrdiff_t>(offset));
        consumed_ += available;
        offset += available;
    }
    return data;
}
Bytes little(std::uint32_t value) {
    return {static_cast<std::uint8_t>(value), static_cast<std::uint8_t>(value >> 8),
            static_cast<std::uint8_t>(value >> 16), static_cast<std::uint8_t>(value >> 24)};
}
std::uint32_t readLittle(std::span<const std::uint8_t> bytes) {
    if (bytes.size() != 4)
        fail("Invalid integer packet");
    return bytes[0] | (std::uint32_t(bytes[1]) << 8) | (std::uint32_t(bytes[2]) << 16) |
           (std::uint32_t(bytes[3]) << 24);
}
Bridge::Bridge(Transport& transport, const Context& context) {
    listener_ = Socket::listen(port_);
    thread_ = std::jthread([this, &transport, deadline = context.deadline](std::stop_token token) {
        try {
            serve(transport, Context{deadline, token, {}});
        } catch (...) {
            if (!token.stop_requested()) {
                std::lock_guard lock(mutex_);
                failure_ = std::current_exception();
            }
        }
    });
}
void Bridge::check() {
    std::lock_guard lock(mutex_);
    if (failure_)
        std::rethrow_exception(failure_);
}
void Bridge::serve(Transport& transport, Context context) {
    for (;;) {
        auto client = listener_.accept(context);
        try {
            for (;;) {
                auto packetContext = context;
                packetContext.deadline = std::min(
                    context.deadline, std::chrono::steady_clock::now() + std::chrono::seconds(10));
                std::string command;
                do {
                    if (command.size() >= 8)
                        fail("Invalid XVC command");
                    command += static_cast<char>(client.receive(1, packetContext)[0]);
                } while (command.back() != ':');
                if (command == "getinfo:")
                    client.send(bytes("xvcServer_v1.0:2048\n"), packetContext);
                else if (command == "settck:") {
                    client.receive(4, packetContext);
                    client.send(little(2133), packetContext);
                } else if (command == "shift:") {
                    const auto count = readLittle(client.receive(4, packetContext));
                    if (!count || count > 16384)
                        fail("Invalid XVC shift length");
                    const auto size = (count + 7) / 8;
                    auto tms = unpack(client.receive(size, packetContext), static_cast<int>(count));
                    auto tdi = unpack(client.receive(size, packetContext), static_cast<int>(count));
                    client.send(pack(transport.shift(tms, tdi)), packetContext);
                } else
                    fail("Invalid XVC command");
            }
        } catch (const ConnectionClosed&) {
            context.check();
        }
    }
}
}
