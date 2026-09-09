#ifndef ONEOCR_H
#define ONEOCR_H
#include <stdint.h>
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif
/* UTF-8 strings. Inputs are borrowed only for each synchronous call.
 * Free every returned non-NULL result/error once with OneOCRFree.
 * Calls on an engine serialize; Close waits for active work. */
typedef enum OneOCRInputKind { ONEOCR_FILE = 1, ONEOCR_ENCODED = 2, ONEOCR_PIXELS = 3 } OneOCRInputKind;
typedef enum OneOCRPixelFormat { ONEOCR_RGB = 1, ONEOCR_RGBA = 2, ONEOCR_BGRA = 3, ONEOCR_RGBX = 4, ONEOCR_BGRX = 5 } OneOCRPixelFormat;
typedef struct OneOCRInput {
    int32_t kind;
    const char *path;
    const uint8_t *data;
    size_t length;
    int32_t width, height, stride, format;
    int32_t premultiplied;
} OneOCRInput;
/* Zero initialization selects defaults. character_classes is a bitmask:
 * han=1, kana=2, hangul=4, latin=8, digits=16; zero enables all five. */
typedef struct OneOCRConfig {
    const char *model_path;
    const char *runtime_library;
    int32_t threads, max_side;
    uint32_t character_classes;
} OneOCRConfig;
/* timeout_ms: 0 = no deadline, maximum 86400000; includes queue time.
 * script: NULL/empty = automatic. Detect ignores script. */
typedef struct OneOCRCallOptions {
    const char *script;
    int64_t timeout_ms;
} OneOCRCallOptions;
#ifndef ONEOCR_IMPLEMENTATION
/* NULL config/options selects defaults. Input structs are never modified. */
uint64_t OneOCROpen(OneOCRConfig *config, char **error);
char *OneOCRRecognize(uint64_t handle, OneOCRInput *input, OneOCRCallOptions *options, char **error);
char *OneOCRDetect(uint64_t handle, OneOCRInput *input, OneOCRCallOptions *options, char **error);
char *OneOCRRecognizeLine(uint64_t handle, OneOCRInput *input, OneOCRCallOptions *options, char **error);
int32_t OneOCRWarmup(uint64_t handle, int64_t timeout_ms, char **error);
char *OneOCRDiagnostics(uint64_t handle, char **error);
int32_t OneOCRClose(uint64_t handle, char **error);
void OneOCRFree(void *value);
#endif
#ifdef __cplusplus
}
#endif
#endif
