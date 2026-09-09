// Build with: go build -buildmode=c-shared -o liboneocr.dylib ./cmd/oneocr-shared
package main

/*
#define ONEOCR_IMPLEMENTATION
#include "../../sdk/include/oneocr.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

//export OneOCROpen
func OneOCROpen(options *C.OneOCRConfig, errorOut **C.char) (result C.uint64_t) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	config := oneocr.Config{}
	if options != nil {
		config.Threads, config.MaxSide = int(options.threads), int(options.max_side)
		config.RuntimeLibrary = goString(options.runtime_library)
		model := goString(options.model_path)
		if stat, err := os.Stat(model); err == nil && stat.IsDir() {
			config.BundleDir = model
		} else {
			config.ModelPath = model
		}
		mask := uint32(options.character_classes)
		if mask & ^uint32(31) != 0 {
			setError(errorOut, fmt.Errorf("oneocr: invalid character class mask"))
			return 0
		}
		for i, class := range []oneocr.CharacterClass{oneocr.CharactersHan, oneocr.CharactersKana, oneocr.CharactersHangul, oneocr.CharactersLatin, oneocr.CharactersDigits} {
			if mask&(1<<i) != 0 {
				config.CharacterClasses = append(config.CharacterClasses, class)
			}
		}
	}
	e, err := oneocr.Open(config)
	if err != nil {
		setError(errorOut, err)
		return 0
	}
	return registerEngine(e)
}
func goString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func registerEngine(e *oneocr.Engine) C.uint64_t {
	handles.Lock()
	id := handles.next
	handles.next++
	handles.engines[id] = e
	handles.Unlock()
	return C.uint64_t(id)
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

func readInput(input *C.OneOCRInput) (oneocr.Input, error) {
	if input == nil {
		return oneocr.Input{}, fmt.Errorf("oneocr: input is required")
	}
	if input.kind == C.ONEOCR_FILE {
		return oneocr.FromFile(goString(input.path)), nil
	}
	limit := uint64(128 * 1024 * 1024)
	if input.kind == C.ONEOCR_PIXELS {
		limit = 256 * 1024 * 1024
	}
	if input.data == nil || input.length == 0 || uint64(input.length) > limit {
		return oneocr.Input{}, fmt.Errorf("oneocr: invalid input buffer length")
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(input.data)), int(input.length))
	switch input.kind {
	case C.ONEOCR_ENCODED:
		return oneocr.FromEncoded(data), nil
	case C.ONEOCR_PIXELS:
		if input.premultiplied != 0 && input.premultiplied != 1 {
			return oneocr.Input{}, fmt.Errorf("oneocr: premultiplied must be 0 or 1")
		}
		if input.format < C.ONEOCR_RGB || input.format > C.ONEOCR_BGRX {
			return oneocr.Input{}, fmt.Errorf("oneocr: invalid pixel format")
		}
		return oneocr.FromPixels(oneocr.Pixels{Data: data, Width: int(input.width), Height: int(input.height), Stride: int(input.stride), Format: oneocr.PixelFormat(input.format), Premultiplied: input.premultiplied != 0}), nil
	default:
		return oneocr.Input{}, fmt.Errorf("oneocr: invalid input kind")
	}
}

func operate(handle C.uint64_t, descriptor *C.OneOCRInput, options *C.OneOCRCallOptions, operation int, errorOut **C.char) (result *C.char) {
	if errorOut != nil {
		*errorOut = nil
	}
	defer guard(errorOut)
	e, err := lookup(handle)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	var timeout C.int64_t
	selected := oneocr.Options{}
	if options != nil {
		timeout = options.timeout_ms
		selected.Script = goString(options.script)
	}
	ctx, cancel, err := timeoutContext(timeout)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	defer cancel()
	input, err := readInput(descriptor)
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	var output interface{}
	switch operation {
	case 0:
		output, err = e.Recognize(ctx, input, selected)
	case 1:
		output, err = e.Detect(ctx, input)
	case 2:
		output, err = e.RecognizeLine(ctx, input, selected)
	}
	if err != nil {
		setError(errorOut, err)
		return nil
	}
	return encodeJSON(output, errorOut)
}

//export OneOCRRecognize
func OneOCRRecognize(handle C.uint64_t, input *C.OneOCRInput, options *C.OneOCRCallOptions, errorOut **C.char) *C.char {
	return operate(handle, input, options, 0, errorOut)
}

//export OneOCRDetect
func OneOCRDetect(handle C.uint64_t, input *C.OneOCRInput, options *C.OneOCRCallOptions, errorOut **C.char) *C.char {
	return operate(handle, input, options, 1, errorOut)
}

//export OneOCRRecognizeLine
func OneOCRRecognizeLine(handle C.uint64_t, input *C.OneOCRInput, options *C.OneOCRCallOptions, errorOut **C.char) *C.char {
	return operate(handle, input, options, 2, errorOut)
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

// OneOCRFree frees only strings returned by this library, including errors.
//
//export OneOCRFree
func OneOCRFree(value unsafe.Pointer) { C.free(value) }

func main() {}
