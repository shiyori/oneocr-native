package engine

import (
	"path"
	"regexp"
	"strings"
)

var acronymBoundary = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
var wordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var unsafeName = regexp.MustCompile(`[^a-z0-9._-]+`)

func resourceSlug(value string) string {
	value = acronymBoundary.ReplaceAllString(value, "${1}_${2}")
	value = wordBoundary.ReplaceAllString(value, "${1}_${2}")
	value = unsafeName.ReplaceAllString(strings.ToLower(value), "_")
	value = strings.Trim(value, "_.-")
	if value == "" {
		return "resource"
	}
	return value
}

// resourceFilename is independent of archive order and training checkpoint.
// OriginalName remains available in bundle.json for exact source provenance.
func resourceFilename(original string) string {
	normalized := strings.ReplaceAll(original, `\`, "/")
	parts := strings.Split(normalized, "/")
	model := strings.HasSuffix(strings.ToLower(normalized), ".onnx")
	for i, part := range parts {
		if strings.EqualFold(part, "Model_Edge") && i+1 < len(parts) {
			parts = parts[i+1:]
			break
		}
	}
	if len(parts) >= 2 {
		role, profile := strings.ToLower(parts[0]), resourceSlug(parts[1])
		file := strings.ToLower(parts[len(parts)-1])
		if model {
			category := map[string]string{"detector": "detection", "character": "recognition", "rejection": "rejection", "confidence": "confidence", "linelayout": "layout"}[role]
			if category != "" {
				return "models/" + category + "/" + profile + ".onnx"
			}
			if role == "auxmltcls" {
				return "models/classification/script_orientation.onnx"
			}
		} else {
			if role == "character" {
				name := map[string]string{"char2ind.txt": "alphabet.txt", "char2inschar.txt": "character_mapping.txt", "composite_chars_map": "composite_characters.txt", "rnn.info": "rnn.info"}[file]
				if name == "" {
					name = resourceSlug(file)
				}
				return "data/recognition/" + profile + "/" + name
			}
			if role == "detector" && file == "checkbox_cal.txt" {
				return "data/detection/" + profile + "/checkbox_calibration.txt"
			}
			if role == "auxmltcls" && file == "handwritten_calibration_map.txt" {
				return "data/classification/handwriting_calibration.txt"
			}
			if role == "enums" {
				return "data/enums/" + resourceSlug(file)
			}
		}
	}
	prefix := "data/"
	if model {
		prefix = "models/"
	}
	return prefix + resourceSlug(path.Base(normalized))
}
