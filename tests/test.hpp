#pragma once
#include <functional>
#include <iostream>
#include <stdexcept>
#include <string>
#include <vector>
#define CHECK(...)                                                                                 \
    do {                                                                                           \
        if (!(__VA_ARGS__))                                                                        \
            throw std::runtime_error(std::string("Check failed: ") + #__VA_ARGS__ + " at line " +  \
                                     std::to_string(__LINE__));                                    \
    } while (false)
namespace test {
template <class F> void rejects(F action) {
    bool thrown = false;
    try {
        action();
    } catch (const std::exception&) {
        thrown = true;
    }
    CHECK(thrown);
}
using Case = std::pair<const char*, std::function<void()>>;
inline int run(const std::vector<Case>& cases) {
    int failed = 0;
    for (const auto& [name, action] : cases) {
        try {
            action();
            std::cout << "PASS " << name << std::endl;
        } catch (const std::exception& error) {
            ++failed;
            std::cerr << "FAIL " << name << ": " << error.what() << '\n';
        }
    }
    std::cout << cases.size() - static_cast<std::size_t>(failed) << " passed, " << failed
              << " failed\n";
    return failed ? 1 : 0;
}
}
