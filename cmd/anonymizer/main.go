package main

import (
	"flag"
	"fmt"
	"os"

	"dicom-anonymizer/internal/cli"
	"dicom-anonymizer/internal/gui"
)

func main() {
	// Define flags
	input := flag.String("input", "", "Input folder containing DICOM files")
	inputShort := flag.String("i", "", "Input folder (shorthand)")

	key := flag.String("key", "", "Secret key for pseudonymization")
	keyShort := flag.String("k", "", "Secret key (shorthand)")

	mapping := flag.String("mapping", "", "Patient mapping file path")
	mappingShort := flag.String("m", "", "Mapping file (shorthand)")

	redactRows := flag.Int("redact-rows", 75, "Rows to redact from ultrasound images")

	recursive := flag.Bool("recursive", true, "Search subdirectories")
	recursiveShort := flag.Bool("r", true, "Recursive (shorthand)")

	retry := flag.Bool("retry", false, "Retry previously failed files")

	metadata := flag.Bool("metadata", true, "Process CT/MRI/X-Ray (metadata only)")
	ultrasound := flag.Bool("ultrasound", true, "Process ultrasound (metadata + pixel redaction)")

	dryRun := flag.Bool("dry-run", false, "Preview only, no files modified")
	dryRunShort := flag.Bool("n", false, "Dry run (shorthand)")

	help := flag.Bool("help", false, "Show help message")
	helpShort := flag.Bool("h", false, "Help (shorthand)")

	// Custom usage message
	flag.Usage = func() {
		cli.PrintUsage()
	}

	flag.Parse()

	// Handle help flag
	if *help || *helpShort {
		cli.PrintUsage()
		return
	}

	// Merge short and long flags (prefer long if both specified)
	inputFolder := *input
	if inputFolder == "" {
		inputFolder = *inputShort
	}

	secretKey := *key
	if secretKey == "" {
		secretKey = *keyShort
	}

	mappingFile := *mapping
	if mappingFile == "" {
		mappingFile = *mappingShort
	}

	isRecursive := *recursive
	if !*recursiveShort {
		isRecursive = false
	}

	isDryRun := *dryRun || *dryRunShort

	// No input folder specified = GUI mode
	if inputFolder == "" {
		app := gui.NewApp()
		app.Run()
		return
	}

	// CLI mode
	opts := cli.Options{
		InputFolder:       inputFolder,
		SecretKey:         secretKey,
		MappingFile:       mappingFile,
		RedactRows:        *redactRows,
		Recursive:         isRecursive,
		RetryFailed:       *retry,
		ProcessMetadata:   *metadata,
		ProcessUltrasound: *ultrasound,
		DryRun:            isDryRun,
	}

	if err := cli.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
