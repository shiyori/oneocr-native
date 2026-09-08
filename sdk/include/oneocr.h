#ifndef ONEOCR_H
#define ONEOCR_H
#include <stdint.h>
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif
/* ABI v1. UTF-8 paths/text. No Go pointers cross this boundary.
 * Every non-NULL result/error must be passed once to OneOCRFree.
 * Input buffers must remain valid for the synchronous call. Never pass a
 * length larger than their allocation. NULL script selects automatic mode.
 * Calls per engine serialize. Close waits for active recognition; callers
 * must stop submitting new work before closing. Different engines may run
 * concurrently. Errors are optional char** outputs, initialized to NULL.
 */
/* model is an .ocrpack file or a legacy bundle directory. A NULL/empty
 * runtime uses ONEOCR_RUNTIME, SDK lib/ discovery or the platform loader. */
uint64_t OneOCROpen(const char *model, const char *runtime, int32_t threads, char **error);
/* Additive ABI v2. JSON fields match Go Config: model_path or bundle_dir,
 * runtime_library, backend (cpu/coreml/cuda/directml), device_id, fallback
 * (cpu/error), threads, cache_dir, adaptation_dir, profiling_dir,
 * coreml_compute_units, stage_backends, shape_cache_size and character_classes. */
uint64_t OneOCROpenWithOptions(const char *options_json, char **error);
/* timeout_ms=0 has no deadline; positive values include time waiting for the
 * engine. Maximum is 86400000. Return 0 on success, -1 on error. */
int32_t OneOCRWarmup(uint64_t handle, int64_t timeout_ms, char **error);
/* Registered providers are not proof of actual acceleration. Profiling is
 * finalized when the engine closes. Returned JSON uses OneOCRFree. */
char *OneOCRDiagnostics(uint64_t handle, char **error);
char *OneOCRRecognizeEncoded(uint64_t handle, const uint8_t *data, size_t length,
                            const char *script, char **error);
char *OneOCRRecognizeRGB(uint64_t handle, const uint8_t *data, size_t length,
                        int32_t width, int32_t height, int32_t stride,
                        const char *script, char **error);
char *OneOCRRecognizeEncodedWithTimeout(uint64_t handle, const uint8_t *data,
                                      size_t length, const char *script,
                                      int64_t timeout_ms, char **error);
char *OneOCRRecognizeRGBWithTimeout(uint64_t handle, const uint8_t *data,
                                  size_t length, int32_t width, int32_t height,
                                  int32_t stride, const char *script,
                                  int64_t timeout_ms, char **error);
/* Independent stages: Detect returns DetectionResult JSON. RecognizeLine takes
 * a cropped horizontal line; NULL script auto-classifies/rotates 180 degrees,
 * explicit script assumes upright input. Both skip full-page recognition. */
char *OneOCRDetectEncoded(uint64_t handle, const uint8_t *data, size_t length, int64_t timeout_ms, char **error);
char *OneOCRDetectRGB(uint64_t handle, const uint8_t *data, size_t length, int32_t width, int32_t height, int32_t stride, int64_t timeout_ms, char **error);
char *OneOCRRecognizeLineEncoded(uint64_t handle, const uint8_t *data, size_t length, const char *script, int64_t timeout_ms, char **error);
char *OneOCRRecognizeLineRGB(uint64_t handle, const uint8_t *data, size_t length, int32_t width, int32_t height, int32_t stride, const char *script, int64_t timeout_ms, char **error);
/* Returns 0 on success, -1 on error. Closing twice reports an invalid handle. */
int32_t OneOCRClose(uint64_t handle, char **error);
/* Closes/invalidate the handle and returns finalized diagnostics JSON. */
char *OneOCRCloseWithDiagnostics(uint64_t handle, char **error);
void OneOCRFree(void *value);
#ifdef __cplusplus
}
#endif
#endif
