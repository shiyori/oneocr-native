package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	oneocr "github.com/shiyori/oneocr-native"
)

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "oneocr:", err)
		os.Exit(1)
	}
}
func emit(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: oneocr install|pack|unpack|export|inspect|recognize|detect|recognize-line (run a command with -h for flags)")
	}
	command := args[0]
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	switch command {
	case "install":
		model := fs.String("model", "", "model .ocrpack or original oneocr.onemodel")
		library := fs.String("runtime", "", "platform ONNX Runtime 1.29 shared library")
		home := fs.String("home", "", "installation root (default ONEOCR_HOME or OS user config)")
		bundle := fs.String("bundle-dir", "", "legacy expanded directory for original OneModel input")
		profile := fs.String("profile", "", "original OneModel conversion: cjk-en (default) or extended")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if *model == "" {
			return fmt.Errorf("--model is required")
		}
		installed, e := oneocr.Install(oneocr.InstallOptions{ModelPath: *model, RuntimeLibrary: *library, Home: *home, BundleDir: *bundle, Profile: *profile})
		if e != nil {
			return e
		}
		return emit(installed)
	case "export":
		model := fs.String("model", "", "source oneocr.onemodel")
		directory := fs.String("directory", "", "new portable bundle directory")
		archive := fs.String("zip", "", "optional ZIP output (must not exist)")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if *model == "" || *directory == "" {
			return fmt.Errorf("--model and --directory are required")
		}
		bundle, e := oneocr.ExportModel(*model, *directory)
		if e != nil {
			return e
		}
		if *archive != "" {
			if e = oneocr.ArchiveBundle(bundle.Directory(), *archive); e != nil {
				return e
			}
		}
		return emit(map[string]any{"bundle": bundle.Directory(), "resources": len(bundle.Resources), "source_sha256": bundle.SourceSHA256, "zip": *archive})
	case "pack":
		model := fs.String("model", "", "original oneocr.onemodel")
		bundle := fs.String("bundle", "", "standard resource directory, alternative to --model")
		profile := fs.String("profile", "cjk-en", "cjk-en or extended")
		output := fs.String("output", "", "new .ocrpack destination")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		info, err := oneocr.Pack(oneocr.PackOptions{ModelPath: *model, BundleDir: *bundle, Profile: *profile, Output: *output})
		if err != nil {
			return err
		}
		return emit(map[string]any{"model": *output, "profile": info.Profile, "resources": len(info.Bundle.Resources)})
	case "unpack":
		model := fs.String("model", "", "input .ocrpack")
		directory := fs.String("directory", "", "new standard bundle directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *model == "" || *directory == "" {
			return fmt.Errorf("--model and --directory are required")
		}
		b, err := oneocr.Unpack(*model, *directory)
		if err != nil {
			return err
		}
		return emit(map[string]any{"bundle": b.Directory(), "resources": len(b.Resources)})
	case "inspect":
		model := fs.String("model", "", "single .ocrpack")
		bundle := fs.String("bundle", "", "legacy bundle directory")
		home := fs.String("home", "", "installation root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *model != "" && *bundle != "" {
			return fmt.Errorf("--model and --bundle are mutually exclusive")
		}
		if *model == "" && *bundle == "" {
			installed, err := oneocr.LoadInstallation(*home)
			if err != nil {
				return err
			}
			*model, *bundle = installed.ModelPath, installed.BundleDir
		}
		if *model != "" {
			info, err := oneocr.ReadPackage(*model)
			if err != nil {
				return err
			}
			return emit(info)
		}
		b, err := oneocr.ReadBundle(*bundle)
		if err != nil {
			return err
		}
		return emit(b)
	case "recognize", "detect", "recognize-line":
		model := fs.String("model", "", "single .ocrpack")
		bundle := fs.String("bundle", "", "portable bundle directory; defaults to installed configuration")
		library := fs.String("runtime", "", "platform ONNX Runtime shared library")
		home := fs.String("home", "", "installation root")
		format := fs.String("format", "text", "text or json")
		script := fs.String("script", "", "optional script override, e.g. CJK")
		threads := fs.Int("threads", 2, "CPU threads per Engine")
		backend := fs.String("backend", "cpu", "cpu, coreml, cuda or directml")
		device := fs.Int("device", 0, "CUDA/DirectML device index")
		fallback := fs.String("fallback", "cpu", "cpu or error; controls whole-session fallback")
		adaptation := fs.String("adaptation-dir", "", "source-verified experimental model set")
		cache := fs.String("cache-dir", "", "CoreML compiled-model cache root")
		compute := fs.String("coreml-compute-units", "ALL", "ALL, CPUOnly, CPUAndGPU or CPUAndNeuralEngine")
		cpuStages := fs.String("cpu-stages", "", "comma-separated stages to keep on CPU, e.g. detector,recognizer/CJK")
		classes := fs.String("characters", "", "comma-separated han,kana,hangul,latin,digits; default all")
		profile := fs.String("profile-dir", "", "optional ORT kernel profiles; finalized after recognition")
		diagnostics := fs.String("diagnostics", "", "optional diagnostics JSON output file")
		warmup := fs.Bool("warmup", false, "load and warm all included recognizers first")
		shapeCache := fs.Int("shape-cache", 0, "accelerated sessions cached per input shape, 0 disables, maximum 4")
		maxSide := fs.Int("max-side", 1600, "maximum detection image side")
		timeout := fs.Duration("timeout", 0, "optional timeout, e.g. 30s")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("%s requires one image filename after all flags", command)
		}
		if *format != "text" && *format != "json" {
			return fmt.Errorf("--format must be text or json")
		}
		if *model != "" && *bundle != "" {
			return fmt.Errorf("--model and --bundle are mutually exclusive")
		}
		if *model == "" && *bundle == "" {
			installed, e := oneocr.LoadInstallation(*home)
			if e != nil {
				return e
			}
			*model, *bundle = installed.ModelPath, installed.BundleDir
			if *library == "" {
				*library = installed.RuntimeLibrary
			}
		}
		config := oneocr.Config{ModelPath: *model, BundleDir: *bundle, RuntimeLibrary: *library, Threads: *threads, MaxSide: *maxSide, Backend: oneocr.Backend(*backend), DeviceID: *device, Fallback: oneocr.FallbackPolicy(*fallback), AdaptationDir: *adaptation, CacheDir: *cache, CoreMLComputeUnits: *compute, ProfilingDir: *profile, ShapeCacheSize: *shapeCache}
		if *cpuStages != "" {
			config.StageBackends = map[string]oneocr.Backend{}
			for _, stage := range strings.Split(*cpuStages, ",") {
				config.StageBackends[strings.TrimSpace(stage)] = oneocr.BackendCPU
			}
		}
		if *classes != "" {
			for _, class := range strings.Split(*classes, ",") {
				config.CharacterClasses = append(config.CharacterClasses, oneocr.CharacterClass(strings.TrimSpace(class)))
			}
		}
		engine, e := oneocr.Open(config)
		if e != nil {
			return e
		}
		defer engine.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if *timeout > 0 {
			var stop context.CancelFunc
			ctx, stop = context.WithTimeout(ctx, time.Duration(*timeout))
			defer stop()
		}
		if *warmup {
			if e = engine.Warmup(ctx); e != nil {
				return e
			}
		}
		var result any
		var resultText string
		switch command {
		case "detect":
			result, e = engine.DetectFile(ctx, fs.Arg(0))
		case "recognize-line":
			var line oneocr.LineResult
			line, e = engine.RecognizeLineFile(ctx, fs.Arg(0), oneocr.Options{Script: *script})
			result, resultText = line, line.Text
		default:
			var full oneocr.Result
			full, e = engine.RecognizeFile(ctx, fs.Arg(0), oneocr.Options{Script: *script})
			result, resultText = full, full.Text
		}
		if closeErr := engine.Close(); e == nil {
			e = closeErr
		}
		if *diagnostics != "" {
			data, err := json.MarshalIndent(engine.Diagnostics(), "", "  ")
			if err != nil {
				return err
			}
			// Diagnostics must not overwrite an existing model or user report.
			f, err := os.OpenFile(*diagnostics, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			_, err = f.Write(append(data, '\n'))
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		if e != nil {
			return e
		}
		if *format == "json" || command == "detect" {
			return emit(result)
		}
		_, e = fmt.Fprintln(os.Stdout, resultText)
		return e
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}
