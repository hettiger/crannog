package main

import (
	"fmt"
	"os"

	"github.com/hettiger/crannog/crannog/cmd"
	"github.com/hettiger/crannog/internal/seatbelt"
)

func main() {
	// Nothing runs before Seatbelt is active (ADR 0030).
	if err := seatbelt.Enter(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	cmd.Execute()
}
