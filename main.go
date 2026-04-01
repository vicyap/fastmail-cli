package main

import (
	"os"

	"github.com/vicyap/fastmail-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
