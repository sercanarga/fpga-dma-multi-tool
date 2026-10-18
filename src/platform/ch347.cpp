#include "platform/timer.hpp"
#include "platform/writer.hpp"
#include <algorithm>
namespace fpga {
namespace {
class Ch347 final : public Transport {
    using Open = int(WINAPI*)(ULONG);
    using Close = BOOL(WINAPI*)(ULONG);
    using Timeout = BOOL(WINAPI*)(ULONG, ULONG, ULONG);
    using Init = BOOL(WINAPI*)(ULONG, UCHAR);
    using Reset = BOOL(WINAPI*)(ULONG, BOOL);
    using Transfer = BOOL(WINAPI*)(ULONG, void*, ULONG*);
    win::Module library_;
    Close close_;
    Transfer read_, write_;
    ULONG index_ = 0;
    bool opened_ = false;
    Context& context_;
    win::ShortWait pacing_{std::chrono::microseconds(250)};

  public:
    Ch347(const Connection& connection, Context& context)
        : library_(connection.library.empty() ? win::runtimePath("drivers/wch/CH347DLLA64.DLL")
                                              : connection.library),
          close_(library_.function<Close>("CH347CloseDevice")),
          read_(library_.function<Transfer>("CH347ReadData")),
          write_(library_.function<Transfer>("CH347WriteData")), context_(context) {
        connection.validate();
        auto open = library_.function<Open>("CH347OpenDevice");
        auto timeout = library_.function<Timeout>("CH347SetTimeout");
        auto init = library_.function<Init>("CH347Jtag_INIT");
        auto reset = library_.function<Reset>("CH347Jtag_ResetTrst");
        try {
            for (int i = connection.adapter < 0 ? 0 : connection.adapter;
                 i <= (connection.adapter < 0 ? 15 : connection.adapter); ++i) {
                context.check();
                if (open(static_cast<ULONG>(i)) == -1)
                    continue;
                if (opened_) {
                    close_(static_cast<ULONG>(i));
                    fail("Multiple CH347 adapters. Select an adapter index.");
                }
                index_ = static_cast<ULONG>(i);
                opened_ = true;
            }
            if (!opened_)
                throw NoAdapter{};
            if (!timeout(index_, 1500, 1500) || !init(index_, 0) || !reset(index_, TRUE))
                fail("CH347 initialization failed");
            pacing_.wait();
        } catch (...) {
            if (opened_)
                close_(index_);
            throw;
        }
    }
    ~Ch347() override {
        if (opened_)
            close_(index_);
    }
    Bits shift(const Bits& tms, const Bits& tdi) override {
        if (tms.empty() || tms.size() != tdi.size() || tms.size() > 16384)
            fail("Invalid CH347 shift");
        Bits result;
        result.reserve(tms.size());
        for (std::size_t offset = 0; offset < tms.size(); offset += 248) {
            context_.check();
            auto count = std::min<std::size_t>(248, tms.size() - offset);
            auto first = static_cast<std::ptrdiff_t>(offset),
                 last = static_cast<std::ptrdiff_t>(offset + count);
            auto command = ch347Command(Bits(tms.begin() + first, tms.begin() + last),
                                        Bits(tdi.begin() + first, tdi.begin() + last));
            auto length = static_cast<ULONG>(command.size());
            if (!write_(index_, command.data(), &length) || length != command.size())
                fail("CH347 write failed");
            Bytes response;
            while (response.size() < count + 3) {
                context_.check();
                Bytes chunk(512);
                length = static_cast<ULONG>(chunk.size());
                if (!read_(index_, chunk.data(), &length) || !length || length > chunk.size())
                    fail("CH347 read failed");
                response.insert(response.end(), chunk.begin(), chunk.begin() + length);
                if (response.size() >= 3 &&
                    (unsigned(response[1]) | (unsigned(response[2]) << 8)) != count)
                    fail("CH347 response length mismatch");
            }
            auto data = ch347Response(response, static_cast<int>(count));
            result.insert(result.end(), data.begin(), data.end());
            pacing_.wait();
        }
        return result;
    }
};
}
std::unique_ptr<Transport> openCh347(const Connection& connection, Context& context) {
    return std::make_unique<Ch347>(connection, context);
}
}
