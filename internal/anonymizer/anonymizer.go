package anonymizer

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	dcm "dicom-anonymizer/internal/dicom"
	"dicom-anonymizer/internal/identity"
	"dicom-anonymizer/internal/progress"
)

// Modality represents the type of DICOM modality
type Modality string

const (
	ModalityMetadata   Modality = "metadata"   // CT/MRI/X-Ray (metadata only)
	ModalityUltrasound Modality = "ultrasound" // Ultrasound (metadata + pixel redaction)
)

// Config holds the anonymization configuration
type Config struct {
	InputFolder       string
	MappingFile       string
	Salt              string
	Modality          Modality
	RedactRows        int
	DryRun            bool
	RetryFailed       bool
	Recursive         bool
	OutputWriter      func(string) // For GUI output
	ProcessMetadata   bool         // Process CT/MRI/X-Ray (metadata only)
	ProcessUltrasound bool         // Process Ultrasound (metadata + pixel redaction)
}

// Stats holds processing statistics
type Stats struct {
	Success         int
	Failed          int
	Skipped         int
	IdentityMatched int
	PIDMatched      int
	TotalPatients   int
}

// PatientGroup represents files grouped by patient
type PatientGroup struct {
	Key   string
	Name  string
	DOB   string
	PID   string
	Files []string
}

// ProcessFolder processes all DICOM files in a folder.
func ProcessFolder(cfg Config) (*Stats, error) {
	output := cfg.OutputWriter
	if output == nil {
		output = func(s string) { fmt.Print(s) }
	}

	inputFolder := cfg.InputFolder
	outputFolder := filepath.Join(inputFolder, "anonymized")

	progressFile := filepath.Join(outputFolder, ".progress.json")
	logFile := filepath.Join(outputFolder, "errors.log")

	// Initialize components
	mapper := identity.NewPseudonymizationMapper(cfg.MappingFile, cfg.Salt)

	var tracker *progress.Tracker
	var errorLogger *progress.ErrorLogger
	var err error

	if !cfg.DryRun {
		tracker = progress.NewTracker(progressFile)
		errorLogger, err = progress.NewErrorLogger(logFile)
		if err != nil {
			return nil, fmt.Errorf("could not create error logger: %w", err)
		}
		defer errorLogger.Close()

		if cfg.RetryFailed {
			tracker.ClearFailed()
		}
	}

	// Find all DICOM files
	files, err := dcm.FindDicomFiles(inputFolder, cfg.Recursive)
	if err != nil {
		return nil, fmt.Errorf("could not find DICOM files: %w", err)
	}

	if len(files) == 0 {
		output(fmt.Sprintf("No DICOM files found in %s\n", inputFolder))
		return &Stats{}, nil
	}

	output(fmt.Sprintf("Found %d DICOM file(s) in %s\n", len(files), inputFolder))

	// Group files by patient identity (Name+DOB) or PatientID
	patients := groupFilesByPatient(files, cfg.Salt, output)
	output(fmt.Sprintf("Found %d unique patient(s)\n", len(patients)))

	if cfg.DryRun {
		return dryRun(patients, mapper, output)
	}

	// Process each patient
	stats := &Stats{}
	var mu sync.Mutex

	for i, patient := range patients {
		anonID, method := mapper.GetAnonID(patient.PID, patient.Name, patient.DOB)

		if method == identity.MatchIdentity {
			stats.IdentityMatched++
		} else {
			stats.PIDMatched++
		}

		patientFolder := filepath.Join(outputFolder, anonID)

		output(fmt.Sprintf("\nProcessing Patient %d/%d\n", i+1, len(patients)))
		if method == identity.MatchIdentity {
			output(fmt.Sprintf("  Name: %s\n", patient.Name))
			output(fmt.Sprintf("  DOB: %s\n", patient.DOB))
		}
		output(fmt.Sprintf("  Original PID: %s\n", patient.PID))
		output(fmt.Sprintf("  Anon ID: %s (%s match)\n", anonID, method))
		output(fmt.Sprintf("  Files: %d\n", len(patient.Files)))

		for _, filePath := range patient.Files {
			if tracker != nil && tracker.IsProcessed(filePath) {
				mu.Lock()
				stats.Skipped++
				mu.Unlock()
				continue
			}

			// Determine output path
			relPath, err := filepath.Rel(inputFolder, filePath)
			if err != nil {
				relPath = filepath.Base(filePath)
			}
			outputPath := filepath.Join(patientFolder, relPath)

			// Process the file - determine method based on modality
			var processErr error

			// Check if this is an ultrasound file
			isUS := false
			if cfg.ProcessUltrasound {
				ds, readErr := dcm.ReadDicomMetadataOnly(filePath)
				if readErr == nil {
					isUS = ds.IsUltrasound()
				}
			}

			if isUS && cfg.ProcessUltrasound {
				processErr = AnonymizeUltrasound(filePath, outputPath, cfg.RedactRows, anonID)
			} else if cfg.ProcessMetadata {
				processErr = AnonymizeMetadata(filePath, outputPath, anonID)
			} else {
				// Skip files that don't match selected modality
				mu.Lock()
				stats.Skipped++
				mu.Unlock()
				continue
			}

			mu.Lock()
			if processErr != nil {
				stats.Failed++
				errMsg := processErr.Error()
				if tracker != nil {
					tracker.MarkError(filePath, errMsg)
				}
				if errorLogger != nil {
					errorLogger.Log(filePath, errMsg)
				}
				output(fmt.Sprintf("  Error: %s: %s\n", filepath.Base(filePath), errMsg))
			} else {
				stats.Success++
				if tracker != nil {
					tracker.MarkSuccess(filePath, outputPath)
				}
			}
			mu.Unlock()
		}
	}

	stats.TotalPatients = len(patients)

	// Print summary
	output(fmt.Sprintf("\n%s\n", strings.Repeat("=", 50)))
	output(fmt.Sprintf("Complete! %d succeeded, %d failed, %d skipped\n",
		stats.Success, stats.Failed, stats.Skipped))
	output(fmt.Sprintf("Matching: %d by Name+DOB, %d by PatientID\n",
		stats.IdentityMatched, stats.PIDMatched))
	if errorLogger != nil {
		output(fmt.Sprintf("  %s\n", errorLogger.Summary()))
	}
	output(fmt.Sprintf("Output: %s\n", outputFolder))
	if cfg.MappingFile != "" {
		output(fmt.Sprintf("Mapping: %s\n", cfg.MappingFile))
	}

	return stats, nil
}

// groupFilesByPatient groups DICOM files by patient identity or ID
func groupFilesByPatient(files []string, salt string, output func(string)) []*PatientGroup {
	patients := make(map[string]*PatientGroup)

	for _, filePath := range files {
		ds, err := dcm.ReadDicomMetadataOnly(filePath)
		if err != nil {
			// Add to UNKNOWN group
			if patients["UNKNOWN"] == nil {
				patients["UNKNOWN"] = &PatientGroup{Key: "UNKNOWN"}
			}
			patients["UNKNOWN"].Files = append(patients["UNKNOWN"].Files, filePath)
			continue
		}

		name := ds.GetPatientName()
		dob := ds.GetPatientBirthDate()
		pid := ds.GetPatientID()
		if pid == "" {
			pid = "UNKNOWN"
		}

		// Create grouping key
		var key string
		if identity.IsValidIdentity(name, dob) {
			key = identity.CreateIdentityHash(name, dob, salt)
		} else {
			key = "PID:" + pid
		}

		if patients[key] == nil {
			patients[key] = &PatientGroup{
				Key:  key,
				Name: name,
				DOB:  dob,
				PID:  pid,
			}
		}
		patients[key].Files = append(patients[key].Files, filePath)
	}

	// Convert map to slice
	result := make([]*PatientGroup, 0, len(patients))
	for _, p := range patients {
		result = append(result, p)
	}

	return result
}

// dryRun performs a dry run, showing what would be processed
func dryRun(patients []*PatientGroup, mapper *identity.PseudonymizationMapper, output func(string)) (*Stats, error) {
	output("\n[DRY RUN] Would process:\n")

	identityCount := 0
	pidCount := 0
	totalFiles := 0

	for _, patient := range patients {
		anonID, method := mapper.GetAnonID(patient.PID, patient.Name, patient.DOB)
		totalFiles += len(patient.Files)

		if method == identity.MatchIdentity {
			identityCount++
			output(fmt.Sprintf("  %s <- '%s' + DOB (%d files) [identity match]\n",
				anonID, patient.Name, len(patient.Files)))
		} else {
			pidCount++
			output(fmt.Sprintf("  %s <- PID '%s' (%d files) [PID fallback]\n",
				anonID, patient.PID, len(patient.Files)))
		}
	}

	output(fmt.Sprintf("\nMatching method: %d by identity, %d by PID\n", identityCount, pidCount))

	return &Stats{
		Skipped:         totalFiles,
		IdentityMatched: identityCount,
		PIDMatched:      pidCount,
		TotalPatients:   len(patients),
	}, nil
}

// ProgressCallback is called during processing to report progress
type ProgressCallback func(current, total int, filename, status string)

// ProcessFolderWithProgress processes all DICOM files with progress callbacks
func ProcessFolderWithProgress(cfg Config, progressCb ProgressCallback) (*Stats, error) {
	output := cfg.OutputWriter
	if output == nil {
		output = func(s string) { fmt.Print(s) }
	}

	inputFolder := cfg.InputFolder
	outputFolder := filepath.Join(inputFolder, "anonymized")

	progressFile := filepath.Join(outputFolder, ".progress.json")
	logFile := filepath.Join(outputFolder, "errors.log")

	// Initialize components
	mapper := identity.NewPseudonymizationMapper(cfg.MappingFile, cfg.Salt)

	var tracker *progress.Tracker
	var errorLogger *progress.ErrorLogger
	var err error

	if !cfg.DryRun {
		tracker = progress.NewTracker(progressFile)
		errorLogger, err = progress.NewErrorLogger(logFile)
		if err != nil {
			return nil, fmt.Errorf("could not create error logger: %w", err)
		}
		defer errorLogger.Close()

		if cfg.RetryFailed {
			tracker.ClearFailed()
		}
	}

	// Find all DICOM files
	files, err := dcm.FindDicomFiles(inputFolder, cfg.Recursive)
	if err != nil {
		return nil, fmt.Errorf("could not find DICOM files: %w", err)
	}

	if len(files) == 0 {
		output(fmt.Sprintf("No DICOM files found in %s\n", inputFolder))
		return &Stats{}, nil
	}

	output(fmt.Sprintf("Found %d DICOM file(s) in %s\n", len(files), inputFolder))

	// Group files by patient identity (Name+DOB) or PatientID
	patients := groupFilesByPatient(files, cfg.Salt, output)
	output(fmt.Sprintf("Found %d unique patient(s)\n", len(patients)))

	if cfg.DryRun {
		return dryRun(patients, mapper, output)
	}

	// Count total files for progress
	totalFiles := 0
	for _, patient := range patients {
		totalFiles += len(patient.Files)
	}

	// Process each patient
	stats := &Stats{}
	var mu sync.Mutex
	fileIndex := 0

	for i, patient := range patients {
		anonID, method := mapper.GetAnonID(patient.PID, patient.Name, patient.DOB)

		if method == identity.MatchIdentity {
			stats.IdentityMatched++
		} else {
			stats.PIDMatched++
		}

		patientFolder := filepath.Join(outputFolder, anonID)

		output(fmt.Sprintf("\nProcessing Patient %d/%d\n", i+1, len(patients)))
		if method == identity.MatchIdentity {
			output(fmt.Sprintf("  Name: %s\n", patient.Name))
			output(fmt.Sprintf("  DOB: %s\n", patient.DOB))
		}
		output(fmt.Sprintf("  Original PID: %s\n", patient.PID))
		output(fmt.Sprintf("  Anon ID: %s (%s match)\n", anonID, method))
		output(fmt.Sprintf("  Files: %d\n", len(patient.Files)))

		for _, filePath := range patient.Files {
			fileIndex++

			if tracker != nil && tracker.IsProcessed(filePath) {
				mu.Lock()
				stats.Skipped++
				mu.Unlock()
				if progressCb != nil {
					progressCb(fileIndex, totalFiles, filepath.Base(filePath), "skipped")
				}
				continue
			}

			// Report progress - processing
			if progressCb != nil {
				progressCb(fileIndex, totalFiles, filepath.Base(filePath), "processing")
			}

			// Determine output path
			relPath, err := filepath.Rel(inputFolder, filePath)
			if err != nil {
				relPath = filepath.Base(filePath)
			}
			outputPath := filepath.Join(patientFolder, relPath)

			// Process the file - determine method based on modality
			var processErr error

			// Check if this is an ultrasound file
			isUS := false
			if cfg.ProcessUltrasound {
				ds, readErr := dcm.ReadDicomMetadataOnly(filePath)
				if readErr == nil {
					isUS = ds.IsUltrasound()
				}
			}

			if isUS && cfg.ProcessUltrasound {
				processErr = AnonymizeUltrasound(filePath, outputPath, cfg.RedactRows, anonID)
			} else if cfg.ProcessMetadata {
				processErr = AnonymizeMetadata(filePath, outputPath, anonID)
			} else {
				// Skip files that don't match selected modality
				mu.Lock()
				stats.Skipped++
				mu.Unlock()
				if progressCb != nil {
					progressCb(fileIndex, totalFiles, filepath.Base(filePath), "skipped")
				}
				continue
			}

			mu.Lock()
			if processErr != nil {
				stats.Failed++
				errMsg := processErr.Error()
				if tracker != nil {
					tracker.MarkError(filePath, errMsg)
				}
				if errorLogger != nil {
					errorLogger.Log(filePath, errMsg)
				}
				output(fmt.Sprintf("  Error: %s: %s\n", filepath.Base(filePath), errMsg))
				if progressCb != nil {
					progressCb(fileIndex, totalFiles, filepath.Base(filePath), "failed")
				}
			} else {
				stats.Success++
				if tracker != nil {
					tracker.MarkSuccess(filePath, outputPath)
				}
				if progressCb != nil {
					progressCb(fileIndex, totalFiles, filepath.Base(filePath), "success")
				}
			}
			mu.Unlock()
		}
	}

	stats.TotalPatients = len(patients)

	// Print summary
	output(fmt.Sprintf("\n%s\n", strings.Repeat("=", 50)))
	output(fmt.Sprintf("Complete! %d succeeded, %d failed, %d skipped\n",
		stats.Success, stats.Failed, stats.Skipped))
	output(fmt.Sprintf("Matching: %d by Name+DOB, %d by PatientID\n",
		stats.IdentityMatched, stats.PIDMatched))
	if errorLogger != nil {
		output(fmt.Sprintf("  %s\n", errorLogger.Summary()))
	}
	output(fmt.Sprintf("Output: %s\n", outputFolder))
	if cfg.MappingFile != "" {
		output(fmt.Sprintf("Mapping: %s\n", cfg.MappingFile))
	}

	return stats, nil
}
