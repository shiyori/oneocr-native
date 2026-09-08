package oneocr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

type shapeSession struct {
	key     string
	network *network
}

// marshalProto preserves every parsed field, including unknown fields and
// tensor payloads. Only graph input dimension messages are changed below.
func marshalProto(fields protoFields) []byte {
	keys := make([]int, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	var out []byte
	for _, key := range keys {
		for _, value := range fields[key] {
			out = binary.AppendUvarint(out, uint64(key<<3|value.wire))
			switch value.wire {
			case 0:
				out = binary.AppendUvarint(out, value.integer)
			case 1, 5:
				out = append(out, value.bytes...)
			case 2:
				out = binary.AppendUvarint(out, uint64(len(value.bytes)))
				out = append(out, value.bytes...)
			}
		}
	}
	return out
}
func protoMessage(fields protoFields) protoValue {
	return protoValue{wire: 2, bytes: marshalProto(fields)}
}
func specializeInputShapes(data []byte, shapes map[string][]int64) ([]byte, error) {
	model, err := parseProto(data)
	if err != nil {
		return nil, err
	}
	graph, err := model.nested(7)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for index, raw := range graph[11] {
		input, err := parseProto(raw.bytes)
		if err != nil {
			return nil, err
		}
		name, err := input.text(1)
		if err != nil {
			return nil, err
		}
		dims, ok := shapes[name]
		if !ok {
			continue
		}
		typ, err := input.nested(2)
		if err != nil {
			return nil, err
		}
		tensor, err := typ.nested(1)
		if err != nil {
			return nil, err
		}
		shape, err := tensor.nested(2)
		if err != nil {
			return nil, err
		}
		if len(shape[1]) != len(dims) {
			return nil, fmt.Errorf("oneocr: input rank mismatch for %s", name)
		}
		for i, size := range dims {
			if size <= 0 {
				return nil, fmt.Errorf("oneocr: invalid input dimension for %s", name)
			}
			dim, err := parseProto(shape[1][i].bytes)
			if err != nil {
				return nil, err
			}
			if len(dim[1]) > 0 {
				fixed, err := dim.number(1, 0)
				if err != nil {
					return nil, err
				}
				if fixed != uint64(size) {
					return nil, fmt.Errorf("oneocr: fixed input dimension mismatch for %s", name)
				}
			}
			delete(dim, 2)
			dim[1] = []protoValue{{wire: 0, integer: uint64(size)}}
			shape[1][i] = protoMessage(dim)
		}
		tensor[2] = []protoValue{protoMessage(shape)}
		typ[1] = []protoValue{protoMessage(tensor)}
		input[2] = []protoValue{protoMessage(typ)}
		graph[11][index] = protoMessage(input)
		found[name] = true
	}
	for name := range shapes {
		if !found[name] {
			return nil, fmt.Errorf("oneocr: missing input %s for shape specialization", name)
		}
	}
	model[7] = []protoValue{protoMessage(graph)}
	return marshalProto(model), nil
}
func inputShapeKey(shapes map[string][]int64) string {
	keys := make([]string, 0, len(shapes))
	for name := range shapes {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, name := range keys {
		out.WriteString(name)
		out.WriteByte('=')
		for _, size := range shapes[name] {
			out.WriteString(strconv.FormatInt(size, 10))
			out.WriteByte('x')
		}
		out.WriteByte(';')
	}
	return out.String()
}

func (n *network) runForShape(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32, allowed []bool, consume func([]ort.Value) error) error {
	shapes := map[string][]int64{"data": data.shape}
	if extraFloat != nil {
		shapes["im_info"] = extraFloat.shape
	}
	if sequence != nil {
		shapes["seq_lengths"] = []int64{1}
	}
	if n.diagnostics.CompactOutput {
		shapes["allowed_tokens"] = []int64{int64(len(allowed))}
	}
	key := inputShapeKey(shapes)
	var selected *network
	for i, s := range n.shapeSessions {
		if s.key == key {
			n.diagnostics.ShapeCacheHits++
			selected = s.network
			copy(n.shapeSessions[i:], n.shapeSessions[i+1:])
			n.shapeSessions[len(n.shapeSessions)-1] = s
			break
		}
	}
	if selected == nil {
		n.diagnostics.ShapeCacheMisses++
		if n.selected == nil {
			return fmt.Errorf("oneocr: unavailable source for shape specialization")
		}
		model, err := n.selected()
		if err != nil {
			return err
		}
		model, err = specializeInputShapes(model, shapes)
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if len(n.shapeSessions) >= n.config.ShapeCacheSize {
			oldest := n.shapeSessions[0]
			n.shapeSessions = n.shapeSessions[1:]
			err = oldest.network.close()
			n.absorbProfile(oldest.network.diagnostics)
			n.diagnostics.ShapeEvictions++
			if err != nil {
				return err
			}
		}
		config := n.config
		config.ShapeCacheSize = 0
		selected = &network{inputs: n.inputs, originalOutputs: n.originalOutputs, config: config, original: n.original, selected: n.selected, diagnostics: StageDiagnostics{Stage: n.diagnostics.Stage, Requested: n.diagnostics.Requested, Recipe: n.diagnostics.Recipe, Experimental: n.diagnostics.Experimental, ShapeKey: key}}
		err = selected.createSession(model, n.diagnostics.Registered, n.outputs)
		if err != nil {
			// Malformed caller shapes/source bytes are rejected above; only provider
			// session creation failures are eligible for original-model CPU fallback.
			if fallbackErr := selected.fallBack(err); fallbackErr != nil {
				return fmt.Errorf("oneocr shape %s: %w", key, fallbackErr)
			}
		}
		n.shapeSessions = append(n.shapeSessions, &shapeSession{key, selected})
		// The original dynamic session was only used to validate registration/model
		// loading. Release it once an actual specialized session has been created.
		if err = n.destroySession(); err != nil {
			return err
		}
	}
	beforeRuns, beforeSeconds := selected.diagnostics.Runs, selected.diagnostics.RunSeconds
	err := selected.runBorrowed(ctx, data, extraFloat, sequence, allowed, consume)
	n.diagnostics.Runs += selected.diagnostics.Runs - beforeRuns
	n.diagnostics.RunSeconds += selected.diagnostics.RunSeconds - beforeSeconds
	return err
}
func (n *network) absorbProfile(s StageDiagnostics) {
	n.diagnostics.ExecutionMeasured = n.diagnostics.ExecutionMeasured || s.ExecutionMeasured
	n.diagnostics.ProfileFiles = append(n.diagnostics.ProfileFiles, s.ProfileFiles...)
	if s.ProfileError != "" {
		n.diagnostics.ProfileError = s.ProfileError
	}
	if n.diagnostics.Providers == nil {
		n.diagnostics.Providers = map[string]ProviderUsage{}
	}
	for name, v := range s.Providers {
		old := n.diagnostics.Providers[name]
		old.KernelEvents += v.KernelEvents
		old.KernelMicroseconds += v.KernelMicroseconds
		n.diagnostics.Providers[name] = old
	}
}
func (n *network) closeAllSessions() error {
	var failures []error
	failures = append(failures, n.destroySession())
	if len(n.shapeSessions) > 0 {
		n.diagnostics.ShapeSessions = nil
	}
	for _, s := range n.shapeSessions {
		failures = append(failures, s.network.close())
		n.absorbProfile(s.network.diagnostics)
		n.diagnostics.ShapeSessions = append(n.diagnostics.ShapeSessions, s.network.snapshotDiagnostics())
	}
	n.shapeSessions = nil
	return errors.Join(failures...)
}
func cloneStageDiagnostics(s StageDiagnostics) StageDiagnostics {
	s.ProfileFiles = append([]string(nil), s.ProfileFiles...)
	providers := make(map[string]ProviderUsage, len(s.Providers))
	for name, v := range s.Providers {
		providers[name] = v
	}
	s.Providers = providers
	shapes := make([]StageDiagnostics, len(s.ShapeSessions))
	for i, v := range s.ShapeSessions {
		shapes[i] = cloneStageDiagnostics(v)
	}
	s.ShapeSessions = shapes
	return s
}
func (n *network) snapshotDiagnostics() StageDiagnostics {
	s := cloneStageDiagnostics(n.diagnostics)
	if len(n.shapeSessions) > 0 {
		s.ShapeSessions = nil
		for _, entry := range n.shapeSessions {
			s.ShapeSessions = append(s.ShapeSessions, entry.network.snapshotDiagnostics())
		}
	}
	return s
}
