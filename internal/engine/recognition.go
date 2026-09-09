package engine

import (
	"bufio"
	"context"
	"fmt"
	ort "github.com/shiyori/oneocr-native/internal/ort"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/bidi"
	"golang.org/x/text/unicode/norm"
)

type alphabet struct {
	characters []string
	blank      int
	composites map[string]string
	allowed    []bool
}

func readAlphabet(data, composites []byte) (*alphabet, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8 alphabet")
	}
	mapping := map[int]string{}
	blank := -1
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		split := strings.LastIndex(line, " ")
		if split <= 0 {
			return nil, fmt.Errorf("invalid alphabet record")
		}
		index, e := strconv.Atoi(line[split+1:])
		if e != nil || index < 0 || index > 1_000_000 {
			return nil, fmt.Errorf("invalid alphabet index")
		}
		if _, exists := mapping[index]; exists {
			return nil, fmt.Errorf("duplicate alphabet index")
		}
		value := line[:split]
		mapping[index] = value
		if value == "<blank>" {
			if blank != -1 {
				return nil, fmt.Errorf("duplicate CTC blank")
			}
			blank = index
		}
	}
	if e := scanner.Err(); e != nil {
		return nil, e
	}
	if blank < 0 {
		return nil, fmt.Errorf("alphabet has no explicit <blank> token")
	}
	a := &alphabet{characters: make([]string, len(mapping)), blank: blank, composites: map[string]string{}}
	for i := range a.characters {
		value, ok := mapping[i]
		if !ok {
			return nil, fmt.Errorf("alphabet indices are not contiguous")
		}
		a.characters[i] = value
	}
	if len(composites) > 0 {
		data = composites
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("invalid composite dictionary UTF-8")
		}
		scanner = bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			key, value, ok := strings.Cut(line, " ")
			if !ok {
				return nil, fmt.Errorf("invalid composite dictionary")
			}
			a.composites[key] = value
		}
		if e := scanner.Err(); e != nil {
			return nil, e
		}
	}
	return a, nil
}

// visualToLogical is a portable inverse-bidi approximation. Unlike reversing
// code points, it preserves combining clusters, Latin phrases and numeric
// runs. Complex inverse bidi is inherently ambiguous; see integration docs.
func visualToLogical(text string) string {
	type cluster struct {
		value string
		kind  int
	}
	var clusters []cluster
	for _, r := range text {
		if (unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r)) && len(clusters) > 0 {
			clusters[len(clusters)-1].value += string(r)
			continue
		}
		p, _ := bidi.LookupRune(r)
		kind := 0
		switch p.Class() {
		case bidi.L:
			kind = 1
		case bidi.EN, bidi.AN:
			kind = 2
		case bidi.R, bidi.AL:
			kind = 3
		}
		clusters = append(clusters, cluster{string(r), kind})
	}
	for i := 0; i < len(clusters); {
		if clusters[i].kind != 0 {
			i++
			continue
		}
		end := i
		space := false
		for end < len(clusters) && clusters[end].kind == 0 {
			for _, r := range clusters[end].value {
				space = space || unicode.IsSpace(r)
			}
			end++
		}
		if i > 0 && end < len(clusters) && clusters[i-1].kind == clusters[end].kind {
			kind := clusters[i-1].kind
			if kind == 1 || kind == 3 || (kind == 2 && !space) {
				for j := i; j < end; j++ {
					clusters[j].kind = kind
				}
			}
		}
		i = end
	}
	type run struct{ start, end, kind int }
	var runs []run
	for i := 0; i < len(clusters); {
		end := i + 1
		for end < len(clusters) && clusters[end].kind == clusters[i].kind {
			end++
		}
		runs = append(runs, run{i, end, clusters[i].kind})
		i = end
	}
	var out strings.Builder
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]
		if r.kind == 1 || r.kind == 2 {
			for j := r.start; j < r.end; j++ {
				out.WriteString(clusters[j].value)
			}
		} else {
			for j := r.end - 1; j >= r.start; j-- {
				base, n := utf8.DecodeRuneInString(clusters[j].value)
				out.WriteString(bidi.ReverseString(string(base)))
				out.WriteString(clusters[j].value[n:])
			}
		}
	}
	return out.String()
}

// RecognitionConfidenceMethod identifies an uncalibrated summary of the model
// posterior at each nonblank CTC emission (consecutive duplicates collapse).
const RecognitionConfidenceMethod = "ctc_token_geometric_mean"

type recognitionResult struct {
	text           string
	logProbability float64
	tokens         int
}

func (r recognitionResult) confidence() *float64 {
	if r.tokens == 0 || r.text == "" {
		return nil
	}
	value := math.Exp(r.logProbability / float64(r.tokens))
	return &value
}

// Normalize across the original alphabet, including disallowed classes. A
// character filter must not inflate the confidence of a low-probability token.
func tokenLogProbability(row []float32, selected int) float64 {
	maximum := float64(row[0])
	for _, value := range row[1:] {
		maximum = math.Max(maximum, float64(value))
	}
	total := 0.0
	for _, value := range row {
		total += math.Exp(float64(value) - maximum)
	}
	return float64(row[selected]) - maximum - math.Log(total)
}

func (a *alphabet) decode(output floatTensor, script string) (string, error) {
	result, err := a.decodeScored(output, script)
	return result.text, err
}

func (a *alphabet) decodeScored(output floatTensor, script string) (recognitionResult, error) {
	var result recognitionResult
	classes := len(a.characters)
	if classes == 0 || len(output.shape) != 3 || output.shape[1] != 1 || output.shape[2] != int64(classes) || output.shape[0] < 0 || len(output.data)%classes != 0 || int64(len(output.data)/classes) != output.shape[0] {
		return result, fmt.Errorf("recognizer output does not match alphabet")
	}
	ids := make([]int64, int(output.shape[0]))
	allowed := a.tokenMask()
	for frame := range ids {
		row := output.data[frame*classes : (frame+1)*classes]
		best := -1
		for i, v := range row {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return result, fmt.Errorf("nonfinite recognizer output")
			}
			if allowed[i] && (best < 0 || v > row[best]) {
				best = i
			}
		}
		if best < 0 {
			return result, fmt.Errorf("empty allowed alphabet")
		}
		ids[frame] = int64(best)
	}
	var err error
	result.text, err = a.decodeIDs(ids, script)
	if err != nil || result.text == "" {
		return result, err
	}
	// Score exactly the emitted tokens, omitting trimmed boundary whitespace.
	type emission struct {
		frame int
		token string
	}
	emissions := []emission{}
	previous := int64(-1)
	for frame, id := range ids {
		if id != previous && id != int64(a.blank) {
			token, err := a.renderToken(id)
			if err != nil {
				return result, err
			}
			if token != "" {
				emissions = append(emissions, emission{frame, token})
			}
		}
		previous = id
	}
	for len(emissions) > 0 && strings.TrimSpace(emissions[0].token) == "" {
		emissions = emissions[1:]
	}
	for len(emissions) > 0 && strings.TrimSpace(emissions[len(emissions)-1].token) == "" {
		emissions = emissions[:len(emissions)-1]
	}
	for _, item := range emissions {
		row := output.data[item.frame*classes : (item.frame+1)*classes]
		result.logProbability += tokenLogProbability(row, int(ids[item.frame]))
		result.tokens++
	}
	return result, nil
}

func (a *alphabet) renderToken(id int64) (string, error) {
	token := a.characters[id]
	switch token {
	case "<space>", "<trash_as_space>":
		token = " "
	default:
		if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
			return "", fmt.Errorf("unsupported alphabet token: %s", token)
		}
	}
	if mapped, ok := a.composites[token]; ok {
		token = mapped
	}
	return token, nil
}
func (a *alphabet) decodeIDs(ids []int64, script string) (string, error) {
	previous := int64(-1)
	var out strings.Builder
	allowed := a.tokenMask()
	for _, best := range ids {
		if best < 0 || best >= int64(len(a.characters)) || !allowed[best] {
			return "", fmt.Errorf("invalid or disallowed recognizer token ID")
		}
		if best != previous && best != int64(a.blank) {
			token, err := a.renderToken(best)
			if err != nil {
				return "", err
			}
			out.WriteString(token)
		}
		previous = best
	}
	text := strings.TrimSpace(out.String())
	if script == "Arabic" || script == "Hebrew" {
		text = visualToLogical(text)
	}
	return norm.NFC.String(text), nil
}

type recognizer struct {
	model    *network
	config   CharacterModel
	alphabet *alphabet
}

func (r *recognizer) run(ctx context.Context, crop raster) (recognitionResult, error) {
	data, e := normalizeLine(crop, r.config.PixelsPerFrame)
	if e != nil {
		return recognitionResult{}, e
	}
	sequence := int32(data.shape[3] / int64(r.config.PixelsPerFrame))
	var result recognitionResult
	e = r.model.runBorrowed(ctx, data, nil, &sequence, func(outputs []ort.Value) error {
		tensor, ok := outputs[0].(*ort.Tensor[float32])
		if !ok {
			return fmt.Errorf("invalid recognizer output type")
		}
		var err error
		result, err = r.alphabet.decodeScored(floatTensor{tensor.GetShape(), tensor.GetData()}, r.config.Script)
		return err
	})
	return result, e
}
