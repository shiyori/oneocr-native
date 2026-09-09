#include <jni.h>
#include <android/bitmap.h>
#include "oneocr.h"
#include <string>
#include <vector>

// JNI modified UTF-8 differs from standard UTF-8 for supplementary characters.
// Construct Java strings through the UTF-8 byte[] constructor instead.
static jstring fromUtf8(JNIEnv *env, const char *value) {
    if (!value) value = "OneOCR failed";
    std::string text(value);
    jbyteArray bytes = env->NewByteArray(static_cast<jsize>(text.size()));
    if (!bytes) return nullptr;
    env->SetByteArrayRegion(bytes, 0, static_cast<jsize>(text.size()), reinterpret_cast<const jbyte *>(text.data()));
    jclass type = env->FindClass("java/lang/String");
    if (!type) { env->DeleteLocalRef(bytes); return nullptr; }
    jmethodID ctor = env->GetMethodID(type, "<init>", "([BLjava/lang/String;)V");
    jstring charset = env->NewStringUTF("UTF-8");
    jstring result = nullptr;
    if (ctor && charset && !env->ExceptionCheck())
        result = static_cast<jstring>(env->NewObject(type, ctor, bytes, charset));
    env->DeleteLocalRef(bytes);
    env->DeleteLocalRef(charset);
    env->DeleteLocalRef(type);
    return result;
}
static void fail(JNIEnv *env, char *error) {
    jstring message = fromUtf8(env, error);
    OneOCRFree(error);
    if (!message || env->ExceptionCheck()) return;
    jclass type = env->FindClass("java/lang/IllegalStateException");
    if (!type) return;
    jmethodID ctor = env->GetMethodID(type, "<init>", "(Ljava/lang/String;)V");
    if (ctor) {
        jthrowable exception = static_cast<jthrowable>(env->NewObject(type, ctor, message));
        if (exception) env->Throw(exception);
    }
}
static std::vector<uint8_t> readBytes(JNIEnv *env, jbyteArray bytes, bool zeroTerminate, jsize maximum = 128 * 1024 * 1024) {
    if (!bytes) return {};
    jsize size = env->GetArrayLength(bytes);
    // Bound before allocation; C ABI enforces the same encoded input limit.
    if (size > maximum) {
        env->ThrowNew(env->FindClass("java/lang/IllegalArgumentException"), "input exceeds byte limit");
        return {};
    }
    std::vector<uint8_t> result(static_cast<size_t>(size) + (zeroTerminate ? 1 : 0));
    if (size) env->GetByteArrayRegion(bytes, 0, size, reinterpret_cast<jbyte *>(result.data()));
    return result;
}
extern "C" JNIEXPORT jlong JNICALL Java_dev_oneocr_OneOcr_nativeOpen(
        JNIEnv *env, jclass, jbyteArray model, jbyteArray runtime, jint threads, jint maxSide, jint classes) {
    auto b = readBytes(env, model, true);
    if (env->ExceptionCheck()) return 0;
    auto r = readBytes(env, runtime, true);
    if (env->ExceptionCheck()) return 0;
    OneOCRConfig config{reinterpret_cast<const char *>(b.data()), reinterpret_cast<const char *>(r.data()), threads, maxSide, static_cast<uint32_t>(classes)};
    char *error = nullptr;
    uint64_t handle = OneOCROpen(&config, &error);
    if (!handle) fail(env, error); else OneOCRFree(error);
    return static_cast<jlong>(handle);
}
static jstring resultString(JNIEnv *env, char *result, char *error) {
    if (!result) { fail(env, error); return nullptr; }
    jstring output = fromUtf8(env, result);
    OneOCRFree(result); OneOCRFree(error); return output;
}
extern "C" JNIEXPORT jstring JNICALL Java_dev_oneocr_OneOcr_nativeOperate(
        JNIEnv *env, jclass, jlong handle, jint kind, jbyteArray path, jbyteArray bytes, jobject bitmap,
        jint width, jint height, jint stride, jint format, jboolean premultiplied,
        jbyteArray script, jlong timeoutMs, jint operation) {
    auto file = readBytes(env, path, true);
    if (env->ExceptionCheck()) return nullptr;
    auto data = readBytes(env, bytes, false, kind == ONEOCR_PIXELS ? 256 * 1024 * 1024 : 128 * 1024 * 1024);
    if (env->ExceptionCheck()) return nullptr;
    auto selected = readBytes(env, script, true);
    if (env->ExceptionCheck()) return nullptr;
    OneOCRInput input{kind, reinterpret_cast<const char *>(file.data()), data.data(), data.size(), width, height, stride, format, premultiplied ? 1 : 0};
    void *pixels = nullptr;
    if (bitmap) {
        AndroidBitmapInfo info{};
        if (AndroidBitmap_getInfo(env, bitmap, &info) != ANDROID_BITMAP_RESULT_SUCCESS || info.format != ANDROID_BITMAP_FORMAT_RGBA_8888 || info.width < 2 || info.height < 2 || uint64_t(info.width) * info.height > 40000000 || info.stride > 256*1024*1024 || uint64_t(info.height) * info.stride > 256*1024*1024) {
            env->ThrowNew(env->FindClass("java/lang/IllegalArgumentException"), "invalid software RGBA bitmap"); return nullptr;
        }
        if (AndroidBitmap_lockPixels(env, bitmap, &pixels) != ANDROID_BITMAP_RESULT_SUCCESS) {
            env->ThrowNew(env->FindClass("java/lang/IllegalArgumentException"), "cannot lock bitmap pixels"); return nullptr;
        }
        input = {ONEOCR_PIXELS, nullptr, static_cast<const uint8_t *>(pixels), size_t(info.stride) * info.height,
            static_cast<int32_t>(info.width), static_cast<int32_t>(info.height), static_cast<int32_t>(info.stride),
            format, premultiplied ? 1 : 0};
    }
    OneOCRCallOptions options{reinterpret_cast<const char *>(selected.data()), timeoutMs};
    char *error = nullptr;
    char *result = operation == 1 ? OneOCRDetect(handle, &input, &options, &error)
        : operation == 2 ? OneOCRRecognizeLine(handle, &input, &options, &error)
        : OneOCRRecognize(handle, &input, &options, &error);
    if (pixels) AndroidBitmap_unlockPixels(env, bitmap);
    return resultString(env, result, error);
}
extern "C" JNIEXPORT void JNICALL Java_dev_oneocr_OneOcr_nativeWarmup(JNIEnv *env, jclass, jlong handle, jlong timeoutMs) {
    char *error = nullptr;
    if (OneOCRWarmup(handle, timeoutMs, &error) != 0) fail(env, error); else OneOCRFree(error);
}
extern "C" JNIEXPORT jstring JNICALL Java_dev_oneocr_OneOcr_nativeDiagnostics(JNIEnv *env, jclass, jlong handle) {
    char *error = nullptr;
    char *result = OneOCRDiagnostics(handle, &error);
    return resultString(env, result, error);
}
extern "C" JNIEXPORT void JNICALL Java_dev_oneocr_OneOcr_nativeClose(JNIEnv *env, jclass, jlong handle) {
    char *error = nullptr;
    if (OneOCRClose(handle, &error) != 0) fail(env, error); else OneOCRFree(error);
}
