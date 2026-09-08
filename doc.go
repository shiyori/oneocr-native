// Package oneocr provides offline OCR with verified ONEOCRPK model packages.
//
// Open accepts Config.ModelPath for a single .ocrpack or Config.BundleDir for
// a legacy resource directory. The engine uses a separately supplied full ONNX
// Runtime CPU library, lazily loads recognizers, and owns its sessions and model
// descriptor until Close. Recognition supports context cancellation.
//
// This is an independent experimental implementation, not full original DLL
// parity. Model resources retain their third-party rights; MIT covers source.
package oneocr
