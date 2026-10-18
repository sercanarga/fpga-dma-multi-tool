#include "core/files.hpp"
#include "core/error.hpp"
#include <fstream>
#include <limits>
namespace fpga {
std::string readFile(const std::filesystem::path& path, std::size_t limit) {
    std::ifstream stream(path, std::ios::binary | std::ios::ate);
    if (!stream)
        fail("Cannot open file: " + path.string());
    const auto length = stream.tellg();
    if (length < 0 || static_cast<std::uintmax_t>(length) > limit ||
        static_cast<std::uintmax_t>(length) >
            static_cast<std::uintmax_t>(std::numeric_limits<std::streamsize>::max()))
        fail("File exceeds read limit: " + path.string());
    std::string data(static_cast<std::size_t>(length), '\0');
    stream.seekg(0);
    if (!stream.read(data.data(), static_cast<std::streamsize>(data.size())) ||
        stream.peek() != std::char_traits<char>::eof())
        fail("File changed while reading: " + path.string());
    return data;
}
}
