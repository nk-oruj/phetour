package main

import (
	"fmt"
	"os"
)

func main() {
	if err := runCommand(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "phetour:", err)
		os.Exit(1)
	}
}
