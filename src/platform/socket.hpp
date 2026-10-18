#pragma once
#include "core/jtag.hpp"
#include "platform/process.hpp"
#include <mutex>
#include <thread>
namespace fpga {
class Socket {
    SOCKET value_ = INVALID_SOCKET;
    Bytes buffered_;
    std::size_t consumed_ = 0;

  public:
    Socket() = default;
    explicit Socket(SOCKET value);
    ~Socket();
    Socket(const Socket&) = delete;
    Socket& operator=(const Socket&) = delete;
    Socket(Socket&& other) noexcept;
    Socket& operator=(Socket&& other) noexcept;
    static Socket listen(std::uint16_t& port);
    Socket accept(Context& context) const;
    void send(std::span<const std::uint8_t> data, Context& context) const;
    Bytes receive(std::size_t count, Context& context);
    void wait(bool writing, Context& context) const;
};
Bytes little(std::uint32_t value);
std::uint32_t readLittle(std::span<const std::uint8_t> bytes);
class Bridge {
    Socket listener_;
    std::uint16_t port_ = 0;
    std::mutex mutex_;
    std::exception_ptr failure_;
    std::jthread thread_;
    void serve(Transport& transport, Context context);

  public:
    Bridge(Transport& transport, const Context& context);
    ~Bridge() = default;
    void check();
    std::uint16_t port() const {
        return port_;
    }
};
}
