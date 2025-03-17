package main

import (
	"fmt"
	"os"

	"github.com/anhinga/anhinga/cmd/anhinga/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}