// Build with: go build -buildmode=c-shared -o liboneocr.dylib ./cmd/oneocr-shared
package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	oneocr "github.com/shiyori/oneocr-native"
)

var handles = struct {
	sync.Mutex
	next    uint64
	engines map[uint64]*oneocr.Engine
}{next: 1, engines: make(map[uint64]*oneocr.Engine)}

func setError(out **C.char, err error) {
	if out != nil && err != nil {
		*out = C.CString(err.Error())
	}
}
func guard(out **C.char) {
	if value := recover(); value != nil {
		setError(out, fmt.Errorf("oneocr: native bridge panic: %v", value))
	}
}
func lookup(handle C.uint64_t) (*oneocr.Engine, error) {
	handles.Lock()
	e := handles.engines[uint64(handle)]
	handles.Unlock()
	if e == nil {
		return nil, fmt.Errorf("oneocr: invalid or closed handle")
	}
	return e, nil
}

// OneOCROpen returns zero on failure. All strings use UTF-8.
//
//export OneOCROpen
func OneOCROpen(bundle, runtime *C.char, threads C.int32_t, errorOut **C.char) (result C.uint64_t) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	if bundle == nil || C.GoString(bundle) == "" {
		setError(errorOut, fmt.Errorf("oneocr: model package or bundle path is required"))
		return 0
	}
	config := oneocr.Config{Threads: int(threads)}
	if runtime != nil {
		config.RuntimeLibrary = C.GoString(runtime)
	}
	source := C.GoString(bundle)
	if stat, err := os.Stat(source); err == nil && stat.IsDir() {
		config.BundleDir = source
	} else {
		config.ModelPath = source
	}
	e, err := oneocr.Open(config)
	if err != nil {
		setError(errorOut, err)
		return 0
	}
	return registerEngine(e)
}

func registerEngine(e *oneocr.Engine) C.uint64_t {
	handles.Lock()
	id := handles.next
	handles.next++
	handles.engines[id] = e
	handles.Unlock()
	return C.uint64_t(id)
}

// OneOCROpenWithOptions adds backend/device/fallback and character configuration.
// It consumes a UTF-8 JSON encoding of oneocr.Config; unknown fields are errors.
//
//export OneOCROpenWithOptions
func OneOCROpenWithOptions(optionsJSON *C.char, errorOut **C.char) (result C.uint64_t) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	if optionsJSON == nil {
		setError(errorOut, fmt.Errorf("oneocr: options JSON is required"))
		return 0
	}
	text := C.GoString(optionsJSON)
	if len(text) > 65536 {
		setError(errorOut, fmt.Errorf("oneocr: options JSON exceeds 64 KiB"))
		return 0
	}
	var config oneocr.Config
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		setError(errorOut, err)
		return 0
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		setError(errorOut, fmt.Errorf("oneocr: trailing options JSON"))
		return 0
	}
	e, err := oneocr.Open(config)
	if err != nil {
		setError(errorOut, err)
		return 0
	}
	return registerEngine(e)
}

func timeoutContext(milliseconds C.int64_t) (context.Context, context.CancelFunc, error) {
	if milliseconds < 0 || milliseconds > 24*60*60*1000 {
		return nil, nil, fmt.Errorf("oneocr: timeout must be 0..86400000 ms")
	}
	if milliseconds == 0 {
		return context.Background(), func() {}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(milliseconds)*time.Millisecond)
	return ctx, cancel, nil
}

//export OneOCRWarmup
func OneOCRWarmup(handle C.uint64_t, timeoutMS C.int64_t, errorOut **C.char) (status C.int32_t) {
	status = -1
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return
	}
	defer cancel()
	if err = e.Warmup(ctx); err != nil {
		setError(errorOut, err)
		return
	}
	return 0
}

//export OneOCRDiagnostics
func OneOCRDiagnostics(handle C.uint64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(e.Diagnostics(), errorOut)
}

func encodeJSON(value interface{}, errorOut **C.char) *C.char {
	data, err := json.Marshal(value)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return C.CString(string(data))
}

func encodeResult(result oneocr.Result, err error, out **C.char) *C.char {
	if err != nil {
		setError(out, err)
		return nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		setError(out, err)
		return nil
	}
	return C.CString(string(data))
}

//export OneOCRRecognizeEncoded
func OneOCRRecognizeEncoded(handle C.uint64_t, data *C.uint8_t, length C.size_t, script *C.char, errorOut **C.char) (result *C.char) {
	return OneOCRRecognizeEncodedWithTimeout(handle, data, length, script, 0, errorOut)
}

//export OneOCRRecognizeEncodedWithTimeout
func OneOCRRecognizeEncodedWithTimeout(handle C.uint64_t, data *C.uint8_t, length C.size_t, script *C.char, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	if data == nil || length == 0 || uint64(length) > 128*1024*1024 {
		setError(errorOut, fmt.Errorf("oneocr: invalid encoded buffer (maximum 128 MiB)"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	option := oneocr.Options{}
	if script != nil {
		option.Script = C.GoString(script)
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	r, err := e.RecognizeEncoded(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)), option)
	return encodeResult(r, err, errorOut)
}

//export OneOCRRecognizeRGB
func OneOCRRecognizeRGB(handle C.uint64_t, data *C.uint8_t, length C.size_t, width, height, stride C.int32_t, script *C.char, errorOut **C.char) (result *C.char) {
	return OneOCRRecognizeRGBWithTimeout(handle, data, length, width, height, stride, script, 0, errorOut)
}

//export OneOCRRecognizeRGBWithTimeout
func OneOCRRecognizeRGBWithTimeout(handle C.uint64_t, data *C.uint8_t, length C.size_t, width, height, stride C.int32_t, script *C.char, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	w, h, s := int64(width), int64(height), int64(stride)
	if data == nil || w < 2 || h < 2 || w*h > 40_000_000 || s < w*3 || uint64(length) > 256*1024*1024 || (h-1)*s+w*3 > int64(length) {
		setError(errorOut, fmt.Errorf("oneocr: invalid RGB buffer dimensions"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	option := oneocr.Options{}
	if script != nil {
		option.Script = C.GoString(script)
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	r, err := e.RecognizeRGB(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)), int(w), int(h), int(s), option)
	return encodeResult(r, err, errorOut)
}

// OneOCRClose removes the handle before waiting for active recognition.
//
//export OneOCRClose
func OneOCRClose(handle C.uint64_t, errorOut **C.char) (status C.int32_t) {
	status = -1
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	handles.Lock()
	e := handles.engines[uint64(handle)]
	delete(handles.engines, uint64(handle))
	handles.Unlock()
	if e == nil {
		setError(errorOut, fmt.Errorf("oneocr: invalid or closed handle"))
		return
	}
	if err := e.Close(); err != nil {
		setError(errorOut, err)
		return
	}
	return 0
}

// OneOCRCloseWithDiagnostics closes the handle and returns finalized profiling.
//
//export OneOCRCloseWithDiagnostics
func OneOCRCloseWithDiagnostics(handle C.uint64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	handles.Lock()
	e := handles.engines[uint64(handle)]
	delete(handles.engines, uint64(handle))
	handles.Unlock()
	if e == nil {
		setError(errorOut, fmt.Errorf("oneocr: invalid or closed handle"))
		return nil
	}
	if err := e.Close(); err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(e.Diagnostics(), errorOut)
}

// OneOCRFree frees only strings returned by this library, including errors.
//
//export OneOCRFree
func OneOCRFree(value unsafe.Pointer) { C.free(value) }

func main() {}

//export OneOCRDetectEncoded
func OneOCRDetectEncoded(handle C.uint64_t, data *C.uint8_t, length C.size_t, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	if data == nil || length == 0 || uint64(length) > 128*1024*1024 {
		setError(errorOut, fmt.Errorf("oneocr: invalid encoded buffer (maximum 128 MiB)"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	r, err := e.DetectEncoded(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)))
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(r, errorOut)
}

//export OneOCRDetectRGB
func OneOCRDetectRGB(handle C.uint64_t, data *C.uint8_t, length C.size_t, width, height, stride C.int32_t, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	w, h, s := int64(width), int64(height), int64(stride)
	if data == nil || w < 2 || h < 2 || w*h > 40_000_000 || s < w*3 || uint64(length) > 256*1024*1024 || (h-1)*s+w*3 > int64(length) {
		setError(errorOut, fmt.Errorf("oneocr: invalid RGB buffer dimensions"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	r, err := e.DetectRGB(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)), int(w), int(h), int(s))
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(r, errorOut)
}

//export OneOCRRecognizeLineEncoded
func OneOCRRecognizeLineEncoded(handle C.uint64_t, data *C.uint8_t, length C.size_t, script *C.char, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	if data == nil || length == 0 || uint64(length) > 128*1024*1024 {
		setError(errorOut, fmt.Errorf("oneocr: invalid encoded buffer (maximum 128 MiB)"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	option := oneocr.Options{}
	if script != nil {
		option.Script = C.GoString(script)
	}
	r, err := e.RecognizeLineEncoded(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)), option)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(r, errorOut)
}

//export OneOCRRecognizeLineRGB
func OneOCRRecognizeLineRGB(handle C.uint64_t, data *C.uint8_t, length C.size_t, width, height, stride C.int32_t, script *C.char, timeoutMS C.int64_t, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	w, h, s := int64(width), int64(height), int64(stride)
	if data == nil || w < 2 || h < 2 || w*h > 40_000_000 || s < w*3 || uint64(length) > 256*1024*1024 || (h-1)*s+w*3 > int64(length) {
		setError(errorOut, fmt.Errorf("oneocr: invalid RGB buffer dimensions"))
		return nil
	}
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	ctx, cancel, err := timeoutContext(timeoutMS)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	option := oneocr.Options{}
	if script != nil {
		option.Script = C.GoString(script)
	}
	r, err := e.RecognizeLineRGB(ctx, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length)), int(w), int(h), int(s), option)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(r, errorOut)
}
