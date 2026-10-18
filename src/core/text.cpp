#include "core/text.hpp"
#include "core/error.hpp"
#include <algorithm>
#include <cctype>
#include <charconv>
#include <iomanip>
#include <sstream>
namespace fpga {
std::string trim(std::string_view text) {
    const auto first = text.find_first_not_of(" \t\r\n");
    return first == text.npos
               ? std::string{}
               : std::string(text.substr(first, text.find_last_not_of(" \t\r\n") - first + 1));
}
std::string lower(std::string text) {
    std::transform(text.begin(), text.end(), text.begin(),
                   [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return text;
}
std::vector<std::string> split(std::string_view text, char separator) {
    std::vector<std::string> result;
    std::size_t start = 0;
    do {
        const auto end = text.find(separator, start);
        result.emplace_back(text.substr(start, end - start));
        if (end == text.npos)
            break;
        start = end + 1;
    } while (true);
    return result;
}
std::string join(const std::vector<std::string>& items, std::string_view separator) {
    std::string result;
    for (const auto& item : items) {
        if (!result.empty())
            result += separator;
        result += item;
    }
    return result;
}
std::string hex(std::uint64_t value, int width) {
    std::ostringstream stream;
    stream << std::uppercase << std::hex << std::setw(width) << std::setfill('0') << value;
    return stream.str();
}
std::uint64_t number(std::string_view text, int base) {
    if (text.starts_with("0x") || text.starts_with("0X")) {
        base = 16;
        text.remove_prefix(2);
    }
    std::uint64_t value{};
    const auto [end, error] = std::from_chars(text.data(), text.data() + text.size(), value, base);
    if (text.empty() || error != std::errc{} || end != text.data() + text.size())
        fail("Invalid number: " + std::string(text));
    return value;
}
std::string decimal(double value, int precision) {
    std::ostringstream stream;
    stream.imbue(std::locale::classic());
    stream << std::fixed << std::setprecision(precision) << value;
    return stream.str();
}
}
