#include <jni.h>
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
static std::vector<uint8_t> readBytes(JNIEnv *env, jbyteArray bytes, bool zeroTerminate) {
    if (!bytes) return {};
    jsize size = env->GetArrayLength(bytes);
    // Bound before allocation; C ABI enforces the same encoded input limit.
    if (size > 128 * 1024 * 1024) {
        env->ThrowNew(env->FindClass("java/lang/IllegalArgumentException"), "input exceeds 128 MiB");
        return {};
    }
    std::vector<uint8_t> result(static_cast<size_t>(size) + (zeroTerminate ? 1 : 0));
    if (size) env->GetByteArrayRegion(bytes, 0, size, reinterpret_cast<jbyte *>(result.data()));
    return result;
}
extern "C" JNIEXPORT jlong JNICALL Java_dev_oneocr_OneOcr_nativeOpen(
        JNIEnv *env, jclass, jbyteArray bundle, jbyteArray runtime, jint threads) {
    auto b = readBytes(env, bundle, true);
    if (env->ExceptionCheck()) return 0;
    auto r = readBytes(env, runtime, true);
    if (env->ExceptionCheck()) return 0;
    char *error = nullptr;
    uint64_t handle = OneOCROpen(reinterpret_cast<const char *>(b.data()), reinterpret_cast<const char *>(r.data()), threads, &error);
    if (!handle) fail(env, error);
    return static_cast<jlong>(handle);
}
extern "C" JNIEXPORT jstring JNICALL Java_dev_oneocr_OneOcr_nativeRecognize(
        JNIEnv *env, jclass, jlong handle, jbyteArray image) {
    auto data = readBytes(env, image, false);
    if (env->ExceptionCheck()) return nullptr;
    char *error = nullptr;
    char *result = OneOCRRecognizeEncoded(static_cast<uint64_t>(handle), data.data(), data.size(), nullptr, &error);
    if (!result) { fail(env, error); return nullptr; }
    jstring output = fromUtf8(env, result);
    OneOCRFree(result);
    return output;
}
extern "C" JNIEXPORT void JNICALL Java_dev_oneocr_OneOcr_nativeClose(JNIEnv *env, jclass, jlong handle) {
    char *error = nullptr;
    if (OneOCRClose(static_cast<uint64_t>(handle), &error) != 0) fail(env, error);
}

extern "C" JNIEXPORT jstring JNICALL Java_dev_oneocr_OneOcr_nativeStage(
        JNIEnv *env, jclass, jlong handle, jbyteArray image, jbyteArray script, jboolean detect) {
    auto data = readBytes(env, image, false);
    if (env->ExceptionCheck()) return nullptr;
    auto selected = readBytes(env, script, true);
    if (env->ExceptionCheck()) return nullptr;
    char *error = nullptr;
    char *result = detect
        ? OneOCRDetectEncoded(static_cast<uint64_t>(handle), data.data(), data.size(), 0, &error)
        : OneOCRRecognizeLineEncoded(static_cast<uint64_t>(handle), data.data(), data.size(),
              selected.empty() ? nullptr : reinterpret_cast<const char *>(selected.data()), 0, &error);
    if (!result) { fail(env, error); return nullptr; }
    jstring output = fromUtf8(env, result);
    OneOCRFree(result);
    OneOCRFree(error);
    return output;
}
