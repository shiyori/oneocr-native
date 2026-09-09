// stream consumes one image path per stdin line and emits NDJSON results.
// Engines are long-lived; the bounded queue prevents unbounded in-process work.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"

	oneocr "github.com/shiyori/oneocr-native"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	model := flag.String("model", "", ".ocrpack file")
	library := flag.String("runtime", "", "ORT library")
	workers := flag.Int("workers", 1, "long-lived Engines (1..4)")
	threads := flag.Int("threads", 1, "CPU threads per Engine (1..16)")
	flag.Parse()
	if *workers < 1 || *workers > 4 {
		return fmt.Errorf("workers must be 1..4")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	engines := make([]*oneocr.Engine, 0, *workers)
	defer func() {
		for _, e := range engines {
			e.Close()
		}
	}()
	for i := 0; i < *workers; i++ {
		e, err := oneocr.Open(oneocr.Config{ModelPath: *model, RuntimeLibrary: *library, Threads: *threads})
		if err != nil {
			return err
		}
		engines = append(engines, e)
		if err = e.Warmup(ctx); err != nil {
			return err
		}
	}
	type request struct {
		ID   int
		Path string
	}
	jobs := make(chan request, *workers*2)
	var wg sync.WaitGroup
	var output sync.Mutex
	encoder := json.NewEncoder(os.Stdout)
	var outputErr error
	for _, e := range engines {
		wg.Add(1)
		go func(e *oneocr.Engine) {
			defer wg.Done()
			for {
				var job request
				select {
				case <-ctx.Done():
					return
				case value, ok := <-jobs:
					if !ok {
						return
					}
					job = value
				}
				result, err := e.Recognize(ctx, oneocr.FromFile(job.Path), oneocr.Options{})
				item := struct {
					ID     int            `json:"id"`
					Path   string         `json:"path"`
					Result *oneocr.Result `json:"result,omitempty"`
					Error  string         `json:"error,omitempty"`
				}{ID: job.ID, Path: job.Path}
				if err != nil {
					item.Error = err.Error()
				} else {
					item.Result = &result
				}
				output.Lock()
				if outputErr == nil {
					outputErr = encoder.Encode(item)
					if outputErr != nil {
						cancel()
					}
				}
				output.Unlock()
			}
		}(e)
	}
	scanned := make(chan error, 1)
	go func() {
		defer close(jobs)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		id := 0
		for scanner.Scan() {
			if scanner.Text() == "" {
				continue
			}
			id++
			select {
			case jobs <- request{id, scanner.Text()}:
			case <-ctx.Done():
				scanned <- ctx.Err()
				return
			}
		}
		scanned <- scanner.Err()
	}()
	var scanErr error
	select {
	case scanErr = <-scanned:
	case <-ctx.Done():
		scanErr = ctx.Err()
	}
	wg.Wait()
	if scanErr != nil {
		return scanErr
	}
	if outputErr != nil {
		return outputErr
	}
	return ctx.Err()
}
