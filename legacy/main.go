package main

import (
	"fmt"
	"os"

	"github.com/projectchimera/chimera/cmd"
)

// main is the entry point for the Chimera CLI.
// It delegates all command routing to the cobra-based cmd package.
func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n[chimera] Fatal error: %v\n", r)
			fmt.Fprintln(os.Stderr, "Please report this issue at https://github.com/projectchimera/chimera/issues")
			os.Exit(1)
		}
	}()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
