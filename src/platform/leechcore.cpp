#include "platform/leechcore.hpp"
#include "core/text.hpp"
#include "platform/leechcore_api.hpp"
#include <cstdarg>
#include <cstddef>
#include <cstdio>
#include <memory>
#include <mutex>
namespace fpga {
namespace {
static_assert(sizeof(LC_CONFIG) == 824);
static_assert(offsetof(LC_CONFIG, paMax) == 536);
static_assert(sizeof(LC_MEMMAP_ENTRY) == 24);
class Diagnostics {
    inline static std::mutex mutex_;
    inline static const Log* output_ = nullptr;
    const Log log_;

  public:
    explicit Diagnostics(Log log) : log_(std::move(log)) {
        std::lock_guard lock(mutex_);
        if (output_)
            fail("A LeechCore diagnostic session is already active");
        output_ = &log_;
    }
    ~Diagnostics() {
        std::lock_guard lock(mutex_);
        output_ = nullptr;
    }
    Diagnostics(const Diagnostics&) = delete;
    Diagnostics& operator=(const Diagnostics&) = delete;
    static int print(const char* format, ...) noexcept {
        va_list arguments;
        va_start(arguments, format);
        va_list copy;
        va_copy(copy, arguments);
        const int size = std::vsnprintf(nullptr, 0, format, copy);
        va_end(copy);
        int result = -1;
        try {
            if (size >= 0) {
                std::vector<char> buffer(static_cast<std::size_t>(size) + 1);
                std::vsnprintf(buffer.data(), buffer.size(), format, arguments);
                std::lock_guard lock(mutex_);
                if (output_ && *output_)
                    (*output_)(std::string(buffer.data(), static_cast<std::size_t>(size)));
                result = size;
            }
        } catch (...) {
        }
        va_end(arguments);
        return result;
    }
};
}
class LeechCore::Implementation {
    Diagnostics diagnostics_;
    win::Module library_{win::runtimePath("leechcore.dll")};
    decltype(&LcClose) close_ = library_.function<decltype(&LcClose)>("LcClose");
    decltype(&LcRead) read_ = library_.function<decltype(&LcRead)>("LcRead");
    decltype(&LcWrite) write_ = library_.function<decltype(&LcWrite)>("LcWrite");
    decltype(&LcCommand) command_ = library_.function<decltype(&LcCommand)>("LcCommand");
    std::unique_ptr<void, decltype(close_)> handle_{nullptr, close_};

  public:
    explicit Implementation(const Benchmark& settings, const Log& log) : diagnostics_(log) {
        const auto create = library_.function<decltype(&LcCreateEx)>("LcCreateEx");
        const auto free = library_.function<decltype(&LcMemFree)>("LcMemFree");
        LC_CONFIG config{};
        config.dwVersion = LC_CONFIG_VERSION;
        config.dwPrintfVerbosity = LC_CONFIG_PRINTF_ENABLED | LC_CONFIG_PRINTF_V;
        config.pfn_printf_opt = Diagnostics::print;
        constexpr char device[] = "fpga://algo=0,readretry=5,tmread=1,tmwrite=1,tmprobe=1";
        std::memcpy(config.szDevice, device, sizeof(device));
        config.paMax =
            settings.address ? settings.address + settings.length - 1 : automaticReadLimit - 1;
        if (log)
            log(std::string("LeechCore device: ") + device + "\n");
        PLC_CONFIG_ERRORINFO errorInfo = nullptr;
        handle_.reset(create(&config, &errorInfo));
        const std::unique_ptr<LC_CONFIG_ERRORINFO, decltype(free)> error(errorInfo, free);
        if (!handle_) {
            std::string message = "Cannot open PCILeech FPGA.";
            if (error && error->dwVersion == LC_CONFIG_ERRORINFO_VERSION &&
                error->cbStruct >= offsetof(LC_CONFIG_ERRORINFO, wszUserText) &&
                error->cwszUserText <=
                    (error->cbStruct - offsetof(LC_CONFIG_ERRORINFO, wszUserText)) /
                        sizeof(WCHAR)) {
                std::wstring_view detail(error->wszUserText, error->cwszUserText);
                while (!detail.empty() && detail.back() == L'\0')
                    detail.remove_suffix(1);
                if (!detail.empty())
                    message += "\n" + win::utf8(detail);
            }
            fail(message);
        }
        restrict(settings.address ? settings.address : automaticReadStride,
                 settings.address ? settings.length : automaticReadLimit - automaticReadStride);
    }
    void restrict(std::uint64_t address, std::uint64_t length) {
        LC_MEMMAP_ENTRY entry{address, length, address};
        if (!command_(handle_.get(), LC_CMD_MEMMAP_SET_STRUCT, sizeof(entry),
                      reinterpret_cast<PBYTE>(&entry), nullptr, nullptr))
            fail("Cannot set the DMA test address range");
    }
    bool tryRead(std::uint64_t address, std::span<std::uint8_t> data) {
        return readPageAligned(address, data, [this](auto base, auto output) {
            return read_(handle_.get(), base, static_cast<DWORD>(output.size()), output.data());
        });
    }
    void read(std::uint64_t address, std::span<std::uint8_t> data) {
        if (!tryRead(address, data))
            fail("DMA read failed at 0x" + hex(address, 16));
    }
    void write(std::uint64_t address, std::span<std::uint8_t> data) {
        if (!write_(handle_.get(), address, static_cast<DWORD>(data.size()), data.data()))
            fail("DMA write failed at 0x" + hex(address, 16));
    }
};
LeechCore::LeechCore(const Benchmark& settings, const Log& log)
    : implementation_(std::make_unique<Implementation>(settings, log)) {}
LeechCore::~LeechCore() = default;
void LeechCore::restrict(std::uint64_t address, std::uint64_t length) {
    implementation_->restrict(address, length);
}
bool LeechCore::tryRead(std::uint64_t address, std::span<std::uint8_t> data) {
    return implementation_->tryRead(address, data);
}
void LeechCore::read(std::uint64_t address, std::span<std::uint8_t> data) {
    implementation_->read(address, data);
}
void LeechCore::write(std::uint64_t address, std::span<std::uint8_t> data) {
    implementation_->write(address, data);
}
}
