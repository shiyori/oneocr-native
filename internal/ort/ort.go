// Package ort is OneOCR's private, dynamically loaded C API 26 adapter.
package ort

/*
#cgo linux LDFLAGS: -ldl
#include "bridge.h"
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

type Runtime struct{ ptr *C.OCRRuntime }

func nativeError(message *C.char) error {
	if message == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(message))
	return fmt.Errorf("%s", C.GoString(message))
}
func LoadedPath() (string, error) {
	var path *C.char
	if err := nativeError(C.ocr_loaded_path(&path)); err != nil {
		return "", err
	}
	if path == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(path))
	return C.GoString(path), nil
}
func Open(path string) (*Runtime, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	r := &Runtime{}
	if err := nativeError(C.ocr_runtime_open(p, &r.ptr)); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Runtime) Version() string { return C.GoString(C.ocr_runtime_version(r.ptr)) }
func (r *Runtime) Close() {
	if r != nil && r.ptr != nil {
		C.ocr_runtime_close(r.ptr)
		r.ptr = nil
	}
}

type Shape []int64

func NewShape(dims ...int64) Shape { return Shape(dims) }

type Value interface {
	Destroy() error
	native() *C.OrtValue
	owner() *Runtime
}
type Tensor[T float32 | int32] struct {
	r     *Runtime
	ptr   *C.OrtValue
	shape Shape
	data  []T
	pin   runtime.Pinner
}

func (t *Tensor[T]) native() *C.OrtValue { return t.ptr }
func (t *Tensor[T]) owner() *Runtime     { return t.r }
func (t *Tensor[T]) GetShape() Shape     { return t.shape }
func (t *Tensor[T]) GetData() []T        { return t.data }
func (t *Tensor[T]) Destroy() error {
	if t.ptr != nil {
		C.ocr_value_close(t.r.ptr, t.ptr)
		t.ptr = nil
		t.pin.Unpin()
		t.data = nil
	}
	return nil
}
func NewTensor[T float32 | int32](r *Runtime, shape Shape, data []T) (*Tensor[T], error) {
	count := int64(1)
	if len(shape) > 16 {
		return nil, fmt.Errorf("oneocr: invalid tensor rank")
	}
	for _, d := range shape {
		if d <= 0 || count > int64(^uint(0)>>1)/d {
			return nil, fmt.Errorf("oneocr: invalid tensor shape")
		}
		count *= d
	}
	if count != int64(len(data)) || len(data) == 0 {
		return nil, fmt.Errorf("oneocr: tensor shape/data mismatch")
	}
	t := &Tensor[T]{r: r, shape: append(Shape(nil), shape...), data: data}
	t.pin.Pin(&data[0])
	kind := C.int(1)
	var z T
	switch any(z).(type) {
	case int32:
		kind = 6
	}
	var shapePtr *C.int64_t
	if len(shape) > 0 {
		shapePtr = (*C.int64_t)(unsafe.Pointer(&shape[0]))
	}
	err := nativeError(C.ocr_tensor(r.ptr, unsafe.Pointer(&data[0]), C.size_t(len(data)*4), shapePtr, C.size_t(len(shape)), kind, &t.ptr))
	if err != nil {
		t.pin.Unpin()
		return nil, err
	}
	return t, nil
}

type Session struct {
	r               *Runtime
	ptr             *C.OrtSession
	inputs, outputs []*C.char
}

func (r *Runtime) NewSession(model []byte, inputs, outputs []string, threads int, androidARM64 bool) (*Session, error) {
	if len(model) == 0 || len(inputs) == 0 || len(outputs) == 0 {
		return nil, fmt.Errorf("oneocr: empty model interface")
	}
	s := &Session{r: r}
	flag := C.int(0)
	if androidARM64 {
		flag = 1
	}
	if err := nativeError(C.ocr_session(r.ptr, unsafe.Pointer(&model[0]), C.size_t(len(model)), C.int(threads), flag, &s.ptr)); err != nil {
		return nil, err
	}
	for _, name := range inputs {
		s.inputs = append(s.inputs, C.CString(name))
	}
	for _, name := range outputs {
		s.outputs = append(s.outputs, C.CString(name))
	}
	return s, nil
}
func (s *Session) Destroy() error {
	if s.ptr != nil {
		C.ocr_session_close(s.r.ptr, s.ptr)
		s.ptr = nil
		for _, p := range s.inputs {
			C.free(unsafe.Pointer(p))
		}
		for _, p := range s.outputs {
			C.free(unsafe.Pointer(p))
		}
	}
	return nil
}

type RunOptions struct {
	r   *Runtime
	ptr *C.OrtRunOptions
}

func (r *Runtime) NewRunOptions() (*RunOptions, error) {
	o := &RunOptions{r: r}
	if err := nativeError(C.ocr_run_options(r.ptr, &o.ptr)); err != nil {
		return nil, err
	}
	return o, nil
}
func (o *RunOptions) Terminate() error { return nativeError(C.ocr_terminate(o.r.ptr, o.ptr)) }
func (o *RunOptions) Destroy() error {
	if o.ptr != nil {
		C.ocr_run_options_close(o.r.ptr, o.ptr)
		o.ptr = nil
	}
	return nil
}
func (s *Session) Run(inputs, outputs []Value) error { return s.RunWithOptions(inputs, outputs, nil) }
func (s *Session) RunWithOptions(inputs, outputs []Value, options *RunOptions) error {
	if len(inputs) != len(s.inputs) || len(outputs) != len(s.outputs) {
		return fmt.Errorf("oneocr: model input/output count mismatch")
	}
	in := make([]*C.OrtValue, len(inputs))
	out := make([]*C.OrtValue, len(outputs))
	for i, v := range inputs {
		if v == nil || v.owner() != s.r || v.native() == nil {
			return fmt.Errorf("oneocr: invalid tensor/runtime ownership")
		}
		in[i] = v.native()
	}
	for _, v := range outputs {
		if v != nil {
			return fmt.Errorf("oneocr: output must be empty")
		}
	}
	var o *C.OrtRunOptions
	if options != nil {
		if options.r != s.r {
			return fmt.Errorf("oneocr: invalid run options ownership")
		}
		o = options.ptr
	}
	err := nativeError(C.ocr_run(s.r.ptr, s.ptr, o, &s.inputs[0], &in[0], C.size_t(len(in)), &s.outputs[0], C.size_t(len(out)), &out[0]))
	defer func() {
		for _, v := range out {
			if v != nil {
				C.ocr_value_close(s.r.ptr, v)
			}
		}
	}()
	if err != nil {
		return err
	}
	for i, v := range out {
		var data unsafe.Pointer
		var dims [16]C.int64_t
		var rank, count C.size_t
		var kind C.int
		if err := nativeError(C.ocr_tensor_info(s.r.ptr, v, &data, &dims[0], &rank, &count, &kind)); err != nil {
			return err
		}
		if kind != 1 || uint64(count) > uint64(^uint(0)>>1)/4 {
			return fmt.Errorf("oneocr: unsupported output tensor")
		}
		shape := make(Shape, int(rank))
		for j := range shape {
			shape[j] = int64(dims[j])
		}
		outputs[i] = &Tensor[float32]{r: s.r, ptr: v, shape: shape, data: unsafe.Slice((*float32)(data), int(count))}
		out[i] = nil
	}
	runtime.KeepAlive(inputs)
	return nil
}

func (r *Runtime) Matches(path string) bool {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.ocr_runtime_matches(r.ptr, p) != 0
}

func ModulePath() string {
	p := C.ocr_module_path()
	if p == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p)
}
