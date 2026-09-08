// Package oneocr provides offline OCR for Chinese, Japanese, Korean and English.
//
// Open(Config{}) discovers the default oneocr-cjk-en.ocrpack model. The engine
// owns its model descriptor and ONNX Runtime CPU sessions until Close.
// RecognizeFile accepts an image path; Recognize and RecognizeRGB accept memory
// inputs. Detect and RecognizeLine are available for separate pipeline stages.
//
// Source license: AGPL-3.0-only. Third-party resources retain their own rights.
package oneocr
