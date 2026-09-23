package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/byx-darwin/ncgo/internal/reference"
)

func main() {
	check := flag.Bool("check", false, "verify generated references without writing")
	flag.Parse()
	root, err := reference.FindRoot(".")
	if err == nil {
		err = reference.Generate(root, *check)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
