package oneocr

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"
)

// The bounded wire reader keeps byte fields as slices. Tensor payloads are
// never interpreted as filesystem paths or executable configuration.
type protoValue struct {
	wire    int
	integer uint64
	bytes   []byte
}
type protoFields map[int][]protoValue

func parseProto(data []byte) (protoFields, error) {
	fields := make(protoFields)
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 || tag>>3 >= 1<<29 {
			return nil, fmt.Errorf("invalid protobuf tag")
		}
		data = data[n:]
		v := protoValue{wire: int(tag & 7)}
		switch v.wire {
		case 0:
			value, n := binary.Uvarint(data)
			if n <= 0 {
				return nil, fmt.Errorf("truncated protobuf varint")
			}
			v.integer = value
			data = data[n:]
		case 1, 5:
			size := 8
			if v.wire == 5 {
				size = 4
			}
			if len(data) < size {
				return nil, fmt.Errorf("truncated protobuf scalar")
			}
			v.bytes = data[:size]
			data = data[size:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				return nil, fmt.Errorf("truncated protobuf bytes")
			}
			data = data[n:]
			v.bytes = data[:int(size)]
			data = data[int(size):]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d", v.wire)
		}
		fields[int(tag>>3)] = append(fields[int(tag>>3)], v)
	}
	return fields, nil
}
func (p protoFields) single(k int) (protoValue, error) {
	v := p[k]
	if len(v) > 1 {
		return protoValue{}, fmt.Errorf("duplicate protobuf field %d", k)
	}
	if len(v) == 0 {
		return protoValue{}, nil
	}
	return v[0], nil
}
func (p protoFields) text(k int) (string, error) {
	v, e := p.single(k)
	if e != nil {
		return "", e
	}
	if len(p[k]) == 0 {
		return "", nil
	}
	if v.wire != 2 || !utf8.Valid(v.bytes) {
		return "", fmt.Errorf("field %d is not UTF-8 text", k)
	}
	return string(v.bytes), nil
}
func (p protoFields) number(k int, def uint64) (uint64, error) {
	v, e := p.single(k)
	if e != nil {
		return 0, e
	}
	if len(p[k]) == 0 {
		return def, nil
	}
	if v.wire != 0 {
		return 0, fmt.Errorf("field %d is not integer", k)
	}
	return v.integer, nil
}
func (p protoFields) nested(k int) (protoFields, error) {
	v, e := p.single(k)
	if e != nil {
		return nil, e
	}
	if len(p[k]) == 0 {
		return make(protoFields), nil
	}
	if v.wire != 2 {
		return nil, fmt.Errorf("field %d is not message", k)
	}
	return parseProto(v.bytes)
}
func (p protoFields) probability(k int, def float64) (float64, error) {
	v, e := p.single(k)
	if e != nil {
		return 0, e
	}
	if len(p[k]) == 0 {
		return def, nil
	}
	var value float64
	switch v.wire {
	case 5:
		value = float64(math.Float32frombits(binary.LittleEndian.Uint32(v.bytes)))
	case 1:
		value = math.Float64frombits(binary.LittleEndian.Uint64(v.bytes))
	default:
		return 0, fmt.Errorf("field %d is not float", k)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 1 {
		return 0, fmt.Errorf("field %d has invalid probability", k)
	}
	return value, nil
}
func packedPath(p protoFields, k int) (string, error) {
	m, e := p.nested(k)
	if e != nil {
		return "", e
	}
	m, e = m.nested(1)
	if e != nil {
		return "", e
	}
	return m.text(1)
}

func parsePipeline(data []byte) (Pipeline, error) {
	var out Pipeline
	p, e := parseProto(data)
	if e != nil {
		return out, e
	}
	d, e := p.nested(1)
	if e != nil {
		return out, e
	}
	c, e := p.nested(20)
	if e != nil {
		return out, e
	}
	dtype, e := d.number(2, 0)
	if e != nil || dtype != 0 {
		return out, fmt.Errorf("unsupported detector architecture")
	}
	ctype, e := c.number(12, 0)
	if e != nil || ctype != 0 {
		return out, fmt.Errorf("unsupported classifier architecture")
	}
	if out.DetectorPath, e = packedPath(d, 3); e != nil {
		return out, e
	}
	if out.ClassifierPath, e = packedPath(c, 2); e != nil {
		return out, e
	}
	if out.SegmentThreshold, e = d.probability(8, .7); e != nil {
		return out, e
	}
	out.LineThresholds = map[int]float64{}
	for _, raw := range d[9] {
		if raw.wire != 2 {
			return out, fmt.Errorf("invalid line thresholds")
		}
		m, e := parseProto(raw.bytes)
		if e != nil {
			return out, e
		}
		name, e := m.text(1)
		if e != nil {
			return out, e
		}
		v, e := m.probability(2, .8)
		if e != nil {
			return out, e
		}
		if name == "P2" || name == "P3" || name == "P4" {
			out.LineThresholds[int(name[1]-'0')] = v
		}
	}
	for _, raw := range p[3] {
		if raw.wire != 2 {
			return out, fmt.Errorf("invalid character model")
		}
		m, e := parseProto(raw.bytes)
		if e != nil {
			return out, e
		}
		kind, e := m.number(2, 0)
		if e != nil || kind >= uint64(len(scripts)) {
			return out, fmt.Errorf("unsupported recognizer architecture")
		}
		stride, e := m.number(7, 0)
		if e != nil || (stride != 4 && stride != 8) {
			return out, fmt.Errorf("unsupported recognizer stride")
		}
		char := CharacterModel{Script: scripts[kind], PixelsPerFrame: int(stride)}
		for _, field := range []struct {
			n     int
			value *string
		}{{1, &char.Name}, {5, &char.AlphabetPath}, {6, &char.PhysicalMapPath}, {12, &char.CompositePath}, {4, &char.PriorPath}} {
			*field.value, e = m.text(field.n)
			if e != nil {
				return out, e
			}
		}
		if char.ModelPath, e = packedPath(m, 3); e != nil {
			return out, e
		}
		out.Characters = append(out.Characters, char)
	}
	return out, validatePipeline(out)
}

func modelInterface(data []byte) (*ModelInterface, error) {
	p, e := parseProto(data)
	if e != nil {
		return nil, e
	}
	graph, e := p.nested(7)
	if e != nil {
		return nil, e
	}
	initializers := map[string]bool{}
	for _, raw := range graph[5] {
		m, e := parseProto(raw.bytes)
		if e != nil {
			return nil, e
		}
		name, e := m.text(8)
		if e != nil {
			return nil, e
		}
		external, e := m.number(14, 0)
		if e != nil || external != 0 || len(m[13]) > 0 {
			return nil, fmt.Errorf("external ONNX data is unsupported")
		}
		initializers[name] = true
	}
	out := &ModelInterface{Inputs: []TensorInfo{}, Outputs: []TensorInfo{}, Opsets: map[string]int{}}
	for _, side := range []struct {
		k   int
		dst *[]TensorInfo
	}{{11, &out.Inputs}, {12, &out.Outputs}} {
		for _, raw := range graph[side.k] {
			m, e := parseProto(raw.bytes)
			if e != nil {
				return nil, e
			}
			name, e := m.text(1)
			if e != nil {
				return nil, e
			}
			if side.k == 11 && initializers[name] {
				continue
			}
			typ, e := m.nested(2)
			if e != nil {
				return nil, e
			}
			tensor, e := typ.nested(1)
			if e != nil {
				return nil, e
			}
			dtype, e := tensor.number(1, 0)
			if e != nil {
				return nil, e
			}
			shape, e := tensor.nested(2)
			if e != nil {
				return nil, e
			}
			t := TensorInfo{Name: name, DType: dtypeName(dtype), Shape: []any{}}
			for _, dim := range shape[1] {
				m, e := parseProto(dim.bytes)
				if e != nil {
					return nil, e
				}
				if len(m[1]) > 0 {
					v, e := m.number(1, 0)
					if e != nil {
						return nil, e
					}
					t.Shape = append(t.Shape, int64(v))
				} else {
					v, e := m.text(2)
					if e != nil {
						return nil, e
					}
					if v == "" {
						t.Shape = append(t.Shape, nil)
					} else {
						t.Shape = append(t.Shape, v)
					}
				}
			}
			*side.dst = append(*side.dst, t)
		}
	}
	operators := map[string]bool{}
	for _, raw := range graph[1] {
		n, e := parseProto(raw.bytes)
		if e != nil {
			return nil, e
		}
		domain, e := n.text(7)
		if e != nil {
			return nil, e
		}
		op, e := n.text(4)
		if e != nil {
			return nil, e
		}
		if domain == "" {
			domain = "ai.onnx"
		}
		operators[domain+"::"+op] = true
	}
	for op := range operators {
		out.Operators = append(out.Operators, op)
	}
	sort.Strings(out.Operators)
	for _, raw := range p[8] {
		m, e := parseProto(raw.bytes)
		if e != nil {
			return nil, e
		}
		domain, e := m.text(1)
		if e != nil {
			return nil, e
		}
		version, e := m.number(2, 0)
		if e != nil {
			return nil, e
		}
		if domain == "" {
			domain = "ai.onnx"
		}
		out.Opsets[domain] = int(version)
	}
	return out, nil
}
func dtypeName(n uint64) string {
	names := []string{"UNDEFINED", "FLOAT", "UINT8", "INT8", "UINT16", "INT16", "INT32", "INT64", "STRING", "BOOL", "FLOAT16", "DOUBLE", "UINT32", "UINT64", "COMPLEX64", "COMPLEX128", "BFLOAT16"}
	if n < uint64(len(names)) {
		return names[n]
	}
	return fmt.Sprintf("TYPE_%d", n)
}
