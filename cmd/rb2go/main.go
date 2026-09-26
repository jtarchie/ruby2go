// Command rb2go compiles a typed Ruby file to Go.
//
//	rb2go [-o out.go] main.rb
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"rb2go"
)

func main() {
	out := flag.String("o", "", "output file (default: stdout)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: rb2go [-o out.go] main.rb")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	src, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code, warnings, err := rb2go.Compile(context.Background(), flag.Arg(0), src)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *out == "" {
		os.Stdout.Write(code)
		return
	}
	if err := os.WriteFile(*out, code, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
