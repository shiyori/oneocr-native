#include "oneocr.hpp"
#ifdef NDEBUG
#undef NDEBUG
#endif
#include <cassert>
#include <cstdlib>
#include <fstream>
#include <iterator>

int main() {
    char *error = nullptr;
    OneOCRConfig invalid{};
    invalid.threads = 17;
    assert(OneOCROpen(&invalid, &error) == 0 && error);
    OneOCRFree(error);
    invalid.threads = 1; invalid.character_classes = 32;
    assert(OneOCROpen(&invalid, &error) == 0 && error);
    OneOCRFree(error);
    assert(OneOCRWarmup(999999, 1, &error) == -1 && error); OneOCRFree(error);
    assert(OneOCRDiagnostics(999999, &error) == nullptr && error); OneOCRFree(error);
    assert(OneOCRClose(999999, &error) == -1 && error); OneOCRFree(error);
    for (auto operation : {OneOCRRecognize, OneOCRDetect, OneOCRRecognizeLine}) {
        assert(operation(999999, nullptr, nullptr, &error) == nullptr && error); OneOCRFree(error);
    }
    const char *model = std::getenv("ONEOCR_BUNDLE");
    const char *runtime = std::getenv("ONEOCR_RUNTIME");
    const char *image = std::getenv("ONEOCR_LINE_IMAGE");
    if (model && runtime && image) {
        oneocr::Options options;
        options.modelPath = model; options.runtimeLibrary = runtime; options.threads = 1;
        oneocr::Engine engine(options);
        std::ifstream in(image, std::ios::binary);
        std::vector<uint8_t> bytes{std::istreambuf_iterator<char>(in), {}};
        assert(!bytes.empty());
        const auto input = oneocr::Input::fromEncoded(bytes);
        const oneocr::CallOptions call{"Latin", 30000};
        assert(engine.detect(input, call).find("\"regions\":[{") != std::string::npos);
        assert(engine.recognizeLine(input, call).find("\"script\":\"Latin\"") != std::string::npos);
        assert(engine.recognize(oneocr::Input::fromFile(image), call).find("Hello World 123") != std::string::npos);
        engine.warmup(30000);
        assert(engine.diagnostics().find("runtime_version") != std::string::npos);
        const uint8_t byte = 1;
        bool rejected = false;
        try { engine.detect(oneocr::Input::fromPixels(&byte, 1, INT32_MAX, INT32_MAX, INT32_MAX)); }
        catch (const std::runtime_error &) { rejected = true; }
        assert(rejected);
        engine.close(); engine.close();
        rejected = false;
        try { engine.recognize(input); } catch (const std::runtime_error &) { rejected = true; }
        assert(rejected);
    }
    OneOCRFree(nullptr);
}
