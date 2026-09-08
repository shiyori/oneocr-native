#include "oneocr.hpp"
#include <iostream>
int main(int argc, char **argv) {
    if (argc != 3 && argc != 4) {
        std::cerr << "usage: oneocr-cpp MODEL IMAGE\n       oneocr-cpp MODEL RUNTIME IMAGE\n";
        return 2;
    }
    try {
        oneocr::Engine engine(argv[1], argc == 4 ? argv[2] : "");
        std::cout << engine.recognizeFile(argv[argc - 1]) << '\n';
        return 0;
    } catch (const std::exception &e) { std::cerr << e.what() << '\n'; return 1; }
}
