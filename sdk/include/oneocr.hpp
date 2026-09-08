#ifndef ONEOCR_HPP
#define ONEOCR_HPP
#include "oneocr.h"
#include <cstdint>
#include <fstream>
#include <memory>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace oneocr {
// Header-only C++17 ownership wrapper. Recognition returns standard UTF-8 JSON.
class Engine {
    uint64_t handle_ = 0;
    struct AdoptHandle {};
    Engine(uint64_t handle, AdoptHandle) noexcept : handle_(handle) {}
    using OwnedString = std::unique_ptr<char, decltype(&OneOCRFree)>;
    [[noreturn]] static void fail(char *error) {
        OwnedString owned(error, OneOCRFree);
        throw std::runtime_error(error ? error : "OneOCR failed without error text");
    }
    static std::string take(char *result, char *error) {
        if (!result) fail(error);
        OwnedString owned(result, OneOCRFree);
        OneOCRFree(error);
        return std::string(result);
    }
    void release() noexcept {
        if (!handle_) return;
        char *error = nullptr;
        OneOCRClose(std::exchange(handle_, 0), &error);
        OneOCRFree(error);
    }
public:
    // JSON follows the Go Config schema; omitted model paths use the default.
    static Engine fromOptions(const std::string &optionsJson) {
        char *error = nullptr;
        uint64_t handle = OneOCROpenWithOptions(optionsJson.c_str(), &error);
        if (!handle) fail(error);
        OneOCRFree(error);
        return Engine(handle, AdoptHandle{});
    }
    explicit Engine(const std::string &model = "", const std::string &runtime = "", int32_t threads = 2) {
        char *error = nullptr;
        handle_ = OneOCROpen(model.c_str(), runtime.empty() ? nullptr : runtime.c_str(), threads, &error);
        if (!handle_) fail(error);
        OneOCRFree(error);
    }
    ~Engine() { release(); }
    Engine(const Engine &) = delete;
    Engine &operator=(const Engine &) = delete;
    Engine(Engine &&other) noexcept : handle_(std::exchange(other.handle_, 0)) {}
    Engine &operator=(Engine &&other) noexcept {
        if (this != &other) { release(); handle_ = std::exchange(other.handle_, 0); }
        return *this;
    }
    void close() {
        if (!handle_) return;
        char *error = nullptr;
        if (OneOCRClose(std::exchange(handle_, 0), &error) != 0) fail(error);
        OneOCRFree(error);
    }
    void warmup(int64_t timeoutMs = 0) const {
        char *error = nullptr;
        if (OneOCRWarmup(handle_, timeoutMs, &error) != 0) fail(error);
        OneOCRFree(error);
    }
    std::string diagnostics() const {
        char *error = nullptr;
        char *result = OneOCRDiagnostics(handle_, &error);
        return take(result, error);
    }
    std::string closeWithDiagnostics() {
        char *error = nullptr;
        char *result = OneOCRCloseWithDiagnostics(std::exchange(handle_, 0), &error);
        return take(result, error);
    }
    std::string recognize(const uint8_t *bytes, size_t size, const std::string &script = "", int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRRecognizeEncodedWithTimeout(handle_, bytes, size, script.empty() ? nullptr : script.c_str(), timeoutMs, &error);
        return take(result, error);
    }
    std::string recognize(const std::vector<uint8_t> &bytes, const std::string &script = "", int64_t timeoutMs = 0) const {
        return recognize(bytes.data(), bytes.size(), script, timeoutMs);
    }
    std::string recognizeFile(const std::string &filename, const std::string &script = "", int64_t timeoutMs = 0) const {
        std::ifstream stream(filename, std::ios::binary | std::ios::ate);
        if (!stream) throw std::runtime_error("cannot open image: " + filename);
        auto size = stream.tellg();
        if (size < 0 || size > 128 * 1024 * 1024) throw std::runtime_error("invalid encoded image size");
        stream.seekg(0);
        std::vector<uint8_t> data(static_cast<size_t>(size));
        if (!data.empty() && !stream.read(reinterpret_cast<char *>(data.data()), size))
            throw std::runtime_error("cannot read image: " + filename);
        return recognize(data, script, timeoutMs);
    }
    std::string recognizeRGB(const uint8_t *bytes, size_t size, int32_t width, int32_t height,
                             int32_t stride, const std::string &script = "", int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRRecognizeRGBWithTimeout(handle_, bytes, size, width, height, stride,
                                          script.empty() ? nullptr : script.c_str(), timeoutMs, &error);
        return take(result, error);
    }
    std::string detect(const uint8_t *bytes, size_t size, int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRDetectEncoded(handle_, bytes, size, timeoutMs, &error);
        return take(result, error);
    }
    std::string detectRGB(const uint8_t *bytes, size_t size, int32_t width, int32_t height, int32_t stride, int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRDetectRGB(handle_, bytes, size, width, height, stride, timeoutMs, &error);
        return take(result, error);
    }
    std::string recognizeLine(const uint8_t *bytes, size_t size, const std::string &script = "", int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRRecognizeLineEncoded(handle_, bytes, size, script.empty() ? nullptr : script.c_str(), timeoutMs, &error);
        return take(result, error);
    }
    std::string recognizeLineRGB(const uint8_t *bytes, size_t size, int32_t width, int32_t height, int32_t stride, const std::string &script = "", int64_t timeoutMs = 0) const {
        char *error = nullptr;
        char *result = OneOCRRecognizeLineRGB(handle_, bytes, size, width, height, stride, script.empty() ? nullptr : script.c_str(), timeoutMs, &error);
        return take(result, error);
    }
};
}
#endif
