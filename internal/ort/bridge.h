#ifndef ONEOCR_ORT_BRIDGE_H
#define ONEOCR_ORT_BRIDGE_H
#include "onnxruntime_c_api.h"
typedef struct OCRRuntime OCRRuntime;
char *ocr_runtime_open(const char *path, OCRRuntime **out);
void ocr_runtime_close(OCRRuntime *r);
int ocr_runtime_matches(OCRRuntime *r, const char *path);
const char *ocr_runtime_version(OCRRuntime *r);
char *ocr_loaded_path(char **out);
char *ocr_module_path(void);
char *ocr_session(OCRRuntime *r, const void *data, size_t size, int threads, int android_arm64, OrtSession **out);
void ocr_session_close(OCRRuntime *r, OrtSession *s);
char *ocr_tensor(OCRRuntime *r, void *data, size_t bytes, const int64_t *shape, size_t rank, int type, OrtValue **out);
void ocr_value_close(OCRRuntime *r, OrtValue *v);
char *ocr_tensor_info(OCRRuntime *r, OrtValue *v, void **data, int64_t *shape, size_t *rank, size_t *count, int *type);
char *ocr_run_options(OCRRuntime *r, OrtRunOptions **out);
char *ocr_terminate(OCRRuntime *r, OrtRunOptions *o);
void ocr_run_options_close(OCRRuntime *r, OrtRunOptions *o);
char *ocr_run(OCRRuntime *r, OrtSession *s, OrtRunOptions *o, const char *const *in_names, const OrtValue *const *inputs, size_t n, const char *const *out_names, size_t m, OrtValue **outputs);
#endif
