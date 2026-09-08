#include "oneocr.hpp"
#ifdef NDEBUG
#undef NDEBUG
#endif
#include <cassert>
#include <cstdint>
#include <cstdlib>
#include <iterator>
int main() {
    char *error = nullptr;
    assert(OneOCROpen(nullptr, nullptr, 2, &error) == 0);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCROpenWithOptions("{\"unknown_field\":true}", &error) == 0);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCROpenWithOptions("{} {}", &error) == 0);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCRWarmup(999999, 1, &error) == -1);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCRDiagnostics(999999, &error) == nullptr);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCRCloseWithDiagnostics(999999, &error) == nullptr);
    assert(error != nullptr);
    OneOCRFree(error);
    const uint8_t byte = 1;
    assert(OneOCRRecognizeEncoded(999999, &byte, 1, nullptr, &error) == nullptr);
    assert(error != nullptr);
    OneOCRFree(error);
    // Dimensions must be rejected before slicing a one-byte input.
    assert(OneOCRRecognizeRGB(999999, &byte, 1, INT32_MAX, INT32_MAX, INT32_MAX, nullptr, &error) == nullptr);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCRClose(999999, &error) == -1);
    assert(error != nullptr);
    OneOCRFree(error);
    assert(OneOCRDetectEncoded(999999, &byte, 1, 0, &error) == nullptr);
    assert(error != nullptr); OneOCRFree(error);
    assert(OneOCRRecognizeLineRGB(999999, &byte, 1, INT32_MAX, INT32_MAX, INT32_MAX, nullptr, 0, &error) == nullptr);
    assert(error != nullptr); OneOCRFree(error);
    const char *model = std::getenv("ONEOCR_BUNDLE");
    const char *runtime = std::getenv("ONEOCR_RUNTIME");
    const char *image = std::getenv("ONEOCR_LINE_IMAGE");
    if (model && runtime && image) {
        // POSIX test fixture paths contain no quotes/backslashes.
        std::string options = "{\"bundle_dir\":\"" + std::string(model) +
            "\",\"runtime_library\":\"" + runtime + "\",\"backend\":\"cpu\"}";
        auto engine = oneocr::Engine::fromOptions(options);
        std::ifstream in(image, std::ios::binary);
        std::vector<uint8_t> bytes{std::istreambuf_iterator<char>(in), {}};
        assert(!bytes.empty());
        auto detection = engine.detect(bytes.data(), bytes.size(), 30000);
        assert(detection.find("\"regions\":[{") != std::string::npos);
        auto line = engine.recognizeLine(bytes.data(), bytes.size(), "Latin", 30000);
        assert(line.find("Hello World 123") != std::string::npos);
        engine.warmup(30000);
        assert(engine.diagnostics().find("CPU") != std::string::npos ||
               engine.diagnostics().find("cpu") != std::string::npos);
        assert(engine.closeWithDiagnostics().find("\"closed\":true") != std::string::npos);
    }
    OneOCRFree(nullptr);
    return 0;
}
