package main

import (
	"fmt"
	"os"
	"remarkable-cli/internal/ctl"
)

func main() {
	if err := ctl.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "remarkablectl:", err)
		os.Exit(1)
	}
}
