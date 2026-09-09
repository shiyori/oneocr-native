#ifndef ONEOCR_HPP
#define ONEOCR_HPP
#include "oneocr.h"
#include <memory>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace oneocr {
enum class PixelFormat { RGB = ONEOCR_RGB, RGBA = ONEOCR_RGBA, BGRA = ONEOCR_BGRA, RGBX = ONEOCR_RGBX, BGRX = ONEOCR_BGRX };
struct Options {
    std::string modelPath, runtimeLibrary;
    int32_t threads = 0, maxSide = 0;
    uint32_t characterClasses = 0;
};
struct CallOptions { std::string script; int64_t timeoutMs = 0; };
// Owns the file path; byte buffers remain borrowed until the call returns.
class Input {
    friend class Engine;
    OneOCRInput value_{};
    std::string path_;
    OneOCRInput descriptor() const { auto v = value_; v.path = path_.c_str(); return v; }
public:
    static Input fromFile(std::string path) { Input i; i.value_.kind = ONEOCR_FILE; i.path_ = std::move(path); return i; }
    static Input fromEncoded(const uint8_t *data, size_t length) { Input i; i.value_.kind = ONEOCR_ENCODED; i.value_.data = data; i.value_.length = length; return i; }
    static Input fromEncoded(const std::vector<uint8_t> &data) { return fromEncoded(data.data(), data.size()); }
    static Input fromEncoded(std::vector<uint8_t> &&) = delete;
    static Input fromPixels(const uint8_t *data, size_t length, int32_t width, int32_t height, int32_t stride, PixelFormat format = PixelFormat::RGB, bool premultiplied = false) {
        Input i; i.value_ = {ONEOCR_PIXELS, nullptr, data, length, width, height, stride, static_cast<int32_t>(format), premultiplied ? 1 : 0}; return i;
    }
};
// C++17 ownership wrapper. Operations return UTF-8 JSON.
class Engine {
    uint64_t handle_ = 0;
    using OwnedString = std::unique_ptr<char, decltype(&OneOCRFree)>;
    [[noreturn]] static void fail(char *error) {
        OwnedString owned(error, OneOCRFree);
        throw std::runtime_error(error ? error : "OneOCR failed without error text");
    }
    static std::string take(char *result, char *error) {
        if (!result) fail(error);
        OwnedString owned(result, OneOCRFree), ownedError(error, OneOCRFree);
        return std::string(result);
    }
    using Operation = char *(*)(uint64_t, OneOCRInput *, OneOCRCallOptions *, char **);
    std::string run(Operation operation, const Input &input, const CallOptions &options) const {
        auto descriptor = input.descriptor();
        OneOCRCallOptions call{options.script.c_str(), options.timeoutMs};
        char *error = nullptr;
        char *result = operation(handle_, &descriptor, &call, &error);
        return take(result, error);
    }
    void release() noexcept {
        if (!handle_) return;
        char *error = nullptr;
        OneOCRClose(std::exchange(handle_, 0), &error);
        OneOCRFree(error);
    }
public:
    explicit Engine(const Options &options = {}) {
        OneOCRConfig config{options.modelPath.c_str(), options.runtimeLibrary.c_str(), options.threads, options.maxSide, options.characterClasses};
        char *error = nullptr;
        handle_ = OneOCROpen(&config, &error);
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
    std::string recognize(const Input &input, const CallOptions &options = {}) const { return run(OneOCRRecognize, input, options); }
    std::string detect(const Input &input, const CallOptions &options = {}) const { return run(OneOCRDetect, input, options); }
    std::string recognizeLine(const Input &input, const CallOptions &options = {}) const { return run(OneOCRRecognizeLine, input, options); }
};
}
#endif
