package engine

import "testing"

func TestStandardResourceNames(t *testing.T) {
	cases := map[string]string{
		`C:\build\Model_Edge\Detector\Universal\ONNX\checkpoint.001.onnx`:             "models/detection/universal.onnx",
		`C:\build\Model_Edge\Character\LatinPrintedV2\ONNX\checkpoint.005_quant.onnx`: "models/recognition/latin_printed_v2.onnx",
		`C:\build\Model_Edge\Character\CJKPrinted\char2ind.txt`:                       "data/recognition/cjk_printed/alphabet.txt",
		`C:\build\Model_Edge\Rejection\CJKMixedDummy\ONNX\dated.onnx`:                 "models/rejection/cjk_mixed_dummy.onnx",
		`C:\build\Model_Edge\Character\HebrewPrinted\composite_chars_map`:             "data/recognition/hebrew_printed/composite_characters.txt",
		`tiny.onnx`: "models/tiny.onnx",
	}
	for original, expected := range cases {
		if actual := resourceFilename(original); actual != expected {
			t.Errorf("%q: got %q, want %q", original, actual, expected)
		}
	}
	// Training checkpoint changes must not alter the public detector path.
	if resourceFilename(`Model_Edge/Detector/Universal/ONNX/checkpoint.999.onnx`) != "models/detection/universal.onnx" {
		t.Fatal("checkpoint leaked into name")
	}
}
