//go:build !experimental

package main

import (
	"fmt"
	"io"
)

func cmdBenchmark(_ []string, _ io.Writer, stderr io.Writer) int {
	fmt.Fprintln(stderr, "benchmark is not available in this build (experimental feature)")
	return 2
}
