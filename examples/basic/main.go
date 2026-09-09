package main

import (
	"context"
	"fmt"
	"os"

	oneocr "github.com/shiyori/oneocr-native"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: basic IMAGE")
		os.Exit(2)
	}
	engine, err := oneocr.Open(oneocr.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer engine.Close()
	result, err := engine.Recognize(context.Background(), oneocr.FromFile(os.Args[1]), oneocr.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(result.Text)
}
