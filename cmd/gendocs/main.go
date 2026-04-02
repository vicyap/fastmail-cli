package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra/doc"
	"github.com/vicyap/fastmail-cli/cmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: gendocs <man|markdown> <output-dir>\n")
		os.Exit(1)
	}

	format := os.Args[1]
	outDir := "."
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("failed to create output dir: %v", err)
	}

	rootCmd := cmd.RootCmd()
	rootCmd.DisableAutoGenTag = true

	switch format {
	case "man":
		header := &doc.GenManHeader{
			Title:   "FM",
			Section: "1",
			Source:  "fm",
			Manual:  "Fastmail CLI",
		}
		if err := doc.GenManTree(rootCmd, header, outDir); err != nil {
			log.Fatalf("failed to generate man pages: %v", err)
		}
	case "markdown":
		if err := doc.GenMarkdownTree(rootCmd, outDir); err != nil {
			log.Fatalf("failed to generate markdown docs: %v", err)
		}
	default:
		log.Fatalf("unknown format: %s (use 'man' or 'markdown')", format)
	}
}
