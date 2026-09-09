#include "oneocr.hpp"
#include <iostream>
#ifdef _WIN32
#include <windows.h>
#endif

static int recognize(const std::string &image) {
    try {
        oneocr::Engine engine;
        std::cout << engine.recognize(oneocr::Input::fromFile(image)) << '\n';
        return 0;
    } catch (const std::exception &error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
#ifdef _WIN32
int wmain(int argc, wchar_t **argv) {
    SetConsoleOutputCP(CP_UTF8);
    if (argc != 2) { std::cerr << "usage: oneocr-cpp IMAGE\n"; return 2; }
    int bytes = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, argv[1], -1, nullptr, 0, nullptr, nullptr);
    if (!bytes) return 2;
    std::string image(static_cast<size_t>(bytes), '\0');
    WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, argv[1], -1, image.data(), bytes, nullptr, nullptr);
    image.pop_back();
    return recognize(image);
}
#else
int main(int argc, char **argv) {
    if (argc != 2) { std::cerr << "usage: oneocr-cpp IMAGE\n"; return 2; }
    return recognize(argv[1]);
}
#endif
