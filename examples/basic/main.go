package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	oneocr "github.com/shiyori/oneocr-native"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "usage: basic [MODEL [RUNTIME]] IMAGE")
		os.Exit(2)
	}
	var engine *oneocr.Engine
	var err error
	if len(os.Args) == 2 {
		engine, err = oneocr.OpenInstalled("")
	} else {
		config := oneocr.Config{Threads: 2}
		if stat, e := os.Stat(os.Args[1]); e == nil && stat.IsDir() {
			config.BundleDir = os.Args[1]
		} else {
			config.ModelPath = os.Args[1]
		}
		if len(os.Args) == 4 {
			config.RuntimeLibrary = os.Args[2]
		}
		engine, err = oneocr.Open(config)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := engine.RecognizeFile(ctx, os.Args[len(os.Args)-1], oneocr.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(result)
}
