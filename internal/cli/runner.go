package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dicom-anonymizer/internal/anonymizer"
	dcm "dicom-anonymizer/internal/dicom"
)

// Options holds CLI configuration options
type Options struct {
	InputFolder       string
	SecretKey         string
	MappingFile       string
	RedactRows        int
	Recursive         bool
	RetryFailed       bool
	ProcessMetadata   bool
	ProcessUltrasound bool
	DryRun            bool
}

// Run executes the CLI anonymization process
func Run(opts Options) error {
	// Check dcmtk status first
	if err := checkDcmtkStatus(); err != nil {
		return err
	}

	// Validate input folder
	if opts.InputFolder == "" {
		return fmt.Errorf("input folder is required")
	}

	// Check if input folder exists
	info, err := os.Stat(opts.InputFolder)
	if err != nil {
		return fmt.Errorf("input folder does not exist: %s", opts.InputFolder)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", opts.InputFolder)
	}

	// Set default mapping file if not specified
	if opts.MappingFile == "" {
		parentDir := filepath.Dir(opts.InputFolder)
		opts.MappingFile = filepath.Join(parentDir, "patient_mapping.json")
	}

	// Generate or validate secret key
	keyGenerated := false
	if opts.SecretKey == "" {
		opts.SecretKey = GenerateSecretKey()
		keyGenerated = true
	}

	// Print header
	printHeader(opts, keyGenerated)

	// Build anonymizer config
	cfg := anonymizer.Config{
		InputFolder:       opts.InputFolder,
		MappingFile:       opts.MappingFile,
		Salt:              opts.SecretKey,
		RedactRows:        opts.RedactRows,
		DryRun:            opts.DryRun,
		RetryFailed:       opts.RetryFailed,
		Recursive:         opts.Recursive,
		ProcessMetadata:   opts.ProcessMetadata,
		ProcessUltrasound: opts.ProcessUltrasound,
		OutputWriter:      func(s string) {}, // Suppress internal output, we use progress callback
	}

	// Create progress bar
	pb := newProgressBar(50)

	// Progress callback
	progressCallback := func(current, total int, filename, status string) {
		pb.update(current, total)
	}

	// Run anonymization
	if opts.DryRun {
		fmt.Println("\n[DRY RUN MODE]")
	}
	fmt.Println()

	stats, err := anonymizer.ProcessFolderWithProgress(cfg, progressCallback)
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	// Print final progress bar at 100%
	if stats.Success > 0 || stats.Failed > 0 || stats.Skipped > 0 {
		total := stats.Success + stats.Failed + stats.Skipped
		pb.update(total, total)
		fmt.Println()
	}

	// Print summary
	printSummary(stats, opts.InputFolder, opts.MappingFile)

	return nil
}

// GenerateSecretKey generates a cryptographically secure 32-character hex key
func GenerateSecretKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// PrintUsage prints CLI usage information
func PrintUsage() {
	fmt.Println(`DICOM Anonymizer - Command Line Interface

USAGE:
  dicom-anonymizer                    Launch GUI (default)
  dicom-anonymizer -i <path> [flags]  Run CLI mode

IMPORTANT - SECRET KEY:
  The secret key (-k) is critical for consistent patient anonymization.

  * You MUST provide the same key when processing different modalities
    (CT, MRI, Ultrasound, X-Ray) for the same patients
  * The key ensures "John Smith" gets the same anonymous ID (e.g., ANON-000001)
    across ALL imaging studies
  * If you lose the key, you cannot maintain patient ID consistency
  * SAVE YOUR KEY SECURELY - store it with your mapping file

FLAGS:
  -i, --input <path>      Input folder containing DICOM files (required for CLI)
  -k, --key <key>         Secret key for pseudonymization (REQUIRED - see above)
                          If not provided, a key is auto-generated and displayed
  -m, --mapping <path>    Patient mapping file (default: {parent}/patient_mapping.json)
                          This file tracks original-to-anonymous ID mappings
      --redact-rows <n>   Rows to redact from ultrasound images (default: 75)
  -r, --recursive         Search subdirectories (default: true)
      --retry             Retry previously failed files from a previous run
      --metadata          Process CT/MRI/X-Ray files (default: true)
      --ultrasound        Process ultrasound with pixel redaction (default: true)
  -n, --dry-run           Preview what will be processed, no files modified
  -h, --help              Show this help message

WORKFLOW - Processing Multiple Modalities:

  1. First run - Generate and SAVE your secret key:
     ./dicom-anonymizer -i /data/CT_Scans -n
     # Note the generated key, e.g.: a1b2c3d4e5f6...

  2. Process CT scans with your key:
     ./dicom-anonymizer -i /data/CT_Scans -k a1b2c3d4e5f6...

  3. Process MRI scans with the SAME key:
     ./dicom-anonymizer -i /data/MRI_Scans -k a1b2c3d4e5f6...

  4. Process Ultrasound with the SAME key:
     ./dicom-anonymizer -i /data/Ultrasound -k a1b2c3d4e5f6...

  Result: Patient "John Smith" will have the same anonymous ID (ANON-XXXXXX)
  across all modalities, enabling cross-modality analysis.

EXAMPLES:
  # Dry run first to preview (recommended)
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY -n

  # Process with your secret key
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY

  # Process only CT/MRI/X-Ray (skip ultrasound)
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --ultrasound=false

  # Process only ultrasound with custom redaction
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --metadata=false --redact-rows=100

  # Retry failed files from previous run
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --retry

  # Use custom mapping file location
  ./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY -m /secure/mappings.json

OUTPUT:
  Anonymized files: {input}/anonymized/ANON-XXXXXX/
  Mapping file:     {parent}/patient_mapping.json (or custom with -m)
  Error log:        {input}/anonymized/errors.log

SECURITY - KEEP THESE SECRET:
  1. Secret Key     - DO NOT share. Required to maintain patient ID consistency.
  2. Mapping File   - DO NOT share. Contains original-to-anonymous ID mappings.
                      Anyone with this file can re-identify patients.

  Only share the anonymized DICOM files in the 'anonymized/' folder.`)
}

// printHeader prints the CLI header with configuration
func printHeader(opts Options, keyGenerated bool) {
	fmt.Println("DICOM Anonymizer")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Input:     %s\n", opts.InputFolder)
	fmt.Printf("Mapping:   %s\n", opts.MappingFile)

	if keyGenerated {
		fmt.Printf("Key:       %s\n", opts.SecretKey)
		fmt.Println()
		fmt.Println("WARNING: Secret key was auto-generated!")
		fmt.Println("         SAVE THIS KEY to maintain consistent patient IDs")
		fmt.Println("         across different imaging modalities (CT, MRI, US, X-Ray).")
		fmt.Println("         Re-run with: -k " + opts.SecretKey)
		fmt.Println()
	} else {
		// Show partial key for security
		if len(opts.SecretKey) > 8 {
			fmt.Printf("Key:       %s... (provided)\n", opts.SecretKey[:8])
		} else {
			fmt.Printf("Key:       %s (provided)\n", opts.SecretKey)
		}
	}

	// Build modality string
	var modalities []string
	if opts.ProcessMetadata {
		modalities = append(modalities, "CT/MRI/X-Ray")
	}
	if opts.ProcessUltrasound {
		modalities = append(modalities, fmt.Sprintf("Ultrasound (%dpx redaction)", opts.RedactRows))
	}
	if len(modalities) == 0 {
		modalities = append(modalities, "None")
	}
	fmt.Printf("Modality:  %s\n", strings.Join(modalities, ", "))

	// Build options string
	var options []string
	if opts.Recursive {
		options = append(options, "Recursive")
	}
	if opts.RetryFailed {
		options = append(options, "Retry failed")
	}
	if opts.DryRun {
		options = append(options, "Dry run")
	}
	if len(options) > 0 {
		fmt.Printf("Options:   %s\n", strings.Join(options, ", "))
	}
}

// printSummary prints the processing summary
func printSummary(stats *anonymizer.Stats, inputFolder, mappingFile string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Complete! %d succeeded, %d failed, %d skipped\n",
		stats.Success, stats.Failed, stats.Skipped)
	fmt.Printf("Patients:  %d total (%d by Name+DOB, %d by PatientID)\n",
		stats.TotalPatients, stats.IdentityMatched, stats.PIDMatched)
	fmt.Printf("Output:    %s\n", filepath.Join(inputFolder, "anonymized"))
	fmt.Printf("Mapping:   %s\n", mappingFile)
}

// progressBar represents a terminal progress bar
type progressBar struct {
	width int
}

// newProgressBar creates a new progress bar with specified width
func newProgressBar(width int) *progressBar {
	return &progressBar{width: width}
}

// update updates the progress bar display
func (pb *progressBar) update(current, total int) {
	if total == 0 {
		return
	}

	percent := float64(current) / float64(total)
	filled := int(percent * float64(pb.width))
	if filled > pb.width {
		filled = pb.width
	}

	bar := strings.Repeat("#", filled) + strings.Repeat("-", pb.width-filled)
	fmt.Printf("\r[%s] %3.0f%%  (%d/%d)", bar, percent*100, current, total)
}

// checkDcmtkStatus checks if dcmtk is installed and prompts for installation if not
func checkDcmtkStatus() error {
	if dcm.CheckDcmtkInstalled() {
		return nil
	}

	fmt.Println("Warning: dcmtk is not installed.")
	fmt.Println("dcmtk is required to process JPEG-LS compressed DICOM files.")
	fmt.Println()

	installCmd := getDcmtkInstallCommand()
	if installCmd == "" {
		fmt.Println("Please install dcmtk using your system package manager and try again.")
		return fmt.Errorf("dcmtk is not installed")
	}

	fmt.Printf("Install command: %s\n", installCmd)
	fmt.Println()
	fmt.Print("Would you like to install dcmtk now? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("dcmtk is not installed")
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Continuing without dcmtk. Some JPEG-LS files may fail to process.")
		return nil
	}

	fmt.Println("Installing dcmtk...")
	cmd := exec.Command("bash", "-lc", installCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Installation failed: %v\n", err)
		fmt.Println("Please install dcmtk manually and try again.")
		return fmt.Errorf("dcmtk installation failed: %w", err)
	}

	// Verify installation
	if !dcm.CheckDcmtkInstalled() {
		fmt.Println("Installation completed but dcmtk is not in PATH.")
		fmt.Println("Please restart your terminal or add dcmtk to your PATH.")
		return fmt.Errorf("dcmtk not found after installation")
	}

	fmt.Println("dcmtk installed successfully!")
	fmt.Println()
	return nil
}

// getDcmtkInstallCommand returns the platform-specific installation command
func getDcmtkInstallCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install dcmtk"
	case "linux":
		return "sudo apt-get update && sudo apt-get install -y dcmtk"
	default:
		return ""
	}
}
