package gui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"dicom-anonymizer/internal/anonymizer"
	dcm "dicom-anonymizer/internal/dicom"
	"dicom-anonymizer/internal/identity"
)

// StepBuilder handles creating UI content for each wizard step
type StepBuilder struct {
	window fyne.Window
	wizard *Wizard

	// Step 1: Input fields
	inputFolderEntry  *widget.Entry
	secretKeyEntry    *widget.Entry
	secretKeyShowBtn  *widget.Button
	secretKeyGenBtn   *widget.Button
	fileCountLabel    *widget.Label

	// Step 2: Settings fields
	metadataCheck     *widget.Check  // CT/MRI/X-Ray
	ultrasoundCheck   *widget.Check  // Ultrasound
	redactRowsEntry   *widget.Entry
	redactRowsLabel   *widget.Label
	redactRowsPixels  *widget.Label
	recursiveCheck    *widget.Check
	mappingFileEntry  *widget.Entry
	retryFailedCheck  *widget.Check

	// Step 3: Preview
	previewProgress    *widget.ProgressBar
	previewStatus      *widget.Label
	previewFilesList   *widget.Label
	previewPatients    *widget.Label
	previewContainer   *fyne.Container
	dryRunComplete     bool
	dryRunStats        *anonymizer.Stats
	patientPreviewData string

	// Step 4: Process
	processProgress    *widget.ProgressBar
	processStatus      *widget.Label
	processFileCount   *widget.Label
	processCurrentFile *widget.Label
	processStats       *widget.Label
	processSummary     *widget.Label
	processContainer   *fyne.Container
	processing         bool
	processingMu       sync.Mutex
}

// NewStepBuilder creates a new step builder
func NewStepBuilder(window fyne.Window, wizard *Wizard) *StepBuilder {
	return &StepBuilder{
		window: window,
		wizard: wizard,
	}
}

// BuildStep1 creates the Input step content
func (s *StepBuilder) BuildStep1() fyne.CanvasObject {
	// Title
	titleLabel := canvas.NewText("Select Input", ColorTextPrimary)
	titleLabel.TextSize = 18
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Input folder
	s.inputFolderEntry = widget.NewEntry()
	s.inputFolderEntry.SetPlaceHolder("/path/to/dicom/files")
	s.inputFolderEntry.OnChanged = func(text string) {
		s.updateFileCount()
		s.autoSetMappingFile()
	}

	inputBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			s.inputFolderEntry.SetText(uri.Path())
		}, s.window)
	})

	inputRow := container.NewBorder(nil, nil, nil, inputBrowseBtn, s.inputFolderEntry)

	// File count indicator
	s.fileCountLabel = widget.NewLabel("")
	s.fileCountLabel.Wrapping = fyne.TextWrapWord

	// Secret key
	s.secretKeyEntry = widget.NewPasswordEntry()
	s.secretKeyEntry.SetPlaceHolder("Enter or generate a secret key")

	s.secretKeyShowBtn = widget.NewButton("Show", func() {
		if s.secretKeyEntry.Password {
			s.secretKeyEntry.Password = false
			s.secretKeyShowBtn.SetText("Hide")
		} else {
			s.secretKeyEntry.Password = true
			s.secretKeyShowBtn.SetText("Show")
		}
		s.secretKeyEntry.Refresh()
	})

	s.secretKeyGenBtn = widget.NewButton("Generate", func() {
		key := s.generateSecretKey()
		s.secretKeyEntry.SetText(key)
		s.secretKeyEntry.Password = false
		s.secretKeyShowBtn.SetText("Hide")
		s.secretKeyEntry.Refresh()

		// Show custom dialog with copy button
		keyLabel := widget.NewLabel(key)
		keyLabel.TextStyle = fyne.TextStyle{Monospace: true}

		copyBtn := widget.NewButton("Copy to Clipboard", func() {
			s.window.Clipboard().SetContent(key)
		})

		content := container.NewVBox(
			widget.NewLabel("Your secret key has been generated:"),
			container.NewCenter(keyLabel),
			container.NewCenter(copyBtn),
			widget.NewSeparator(),
			widget.NewLabel("IMPORTANT: Save this key securely!\nYou will need it to:"),
			widget.NewLabel("  • De-anonymize patients in the future"),
			widget.NewLabel("  • Maintain consistent IDs across DICOM files"),
		)

		d := dialog.NewCustom("Save Your Secret Key", "OK", content, s.window)
		d.Resize(fyne.NewSize(450, 280))
		d.Show()
	})

	keyButtonsRow := container.NewHBox(s.secretKeyShowBtn, s.secretKeyGenBtn)
	secretKeyRow := container.NewBorder(nil, nil, nil, keyButtonsRow, s.secretKeyEntry)

	// Secret key explanation
	keyExplanation := widget.NewLabel("Required for de-anonymization and consistent patient IDs across files")
	keyExplanation.Wrapping = fyne.TextWrapWord

	// Build form
	content := container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabelWithStyle("Input Folder Path", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			inputRow,
			s.fileCountLabel,
		),
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabelWithStyle("Secret Key", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			keyExplanation,
			secretKeyRow,
		),
	)

	return container.NewPadded(content)
}

// generateSecretKey generates a cryptographically secure random key
func (s *StepBuilder) generateSecretKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// BuildStep2 creates the Settings step content
func (s *StepBuilder) BuildStep2() fyne.CanvasObject {
	// Title
	titleLabel := canvas.NewText("Configure Settings", ColorTextPrimary)
	titleLabel.TextSize = 18
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Redact rows settings (create first so callback can access them)
	s.redactRowsLabel = widget.NewLabelWithStyle("Redact rows from top:", fyne.TextAlignLeading, fyne.TextStyle{})
	s.redactRowsEntry = widget.NewEntry()
	s.redactRowsEntry.SetText("75")
	s.redactRowsEntry.SetPlaceHolder("75")
	s.redactRowsPixels = widget.NewLabel("pixels")

	redactRow := container.NewHBox(
		s.redactRowsLabel,
		s.redactRowsEntry,
		s.redactRowsPixels,
	)

	// Modality selection - checkboxes
	s.metadataCheck = widget.NewCheck("CT / MRI / X-Ray (metadata only)", nil)
	s.metadataCheck.SetChecked(true) // Default selected

	s.ultrasoundCheck = widget.NewCheck("Ultrasound (metadata + pixel redaction)", func(checked bool) {
		if checked {
			s.redactRowsLabel.Show()
			s.redactRowsEntry.Show()
			s.redactRowsPixels.Show()
		} else {
			s.redactRowsLabel.Hide()
			s.redactRowsEntry.Hide()
			s.redactRowsPixels.Hide()
		}
	})
	s.ultrasoundCheck.SetChecked(true) // Default selected (shows redact rows)

	// Recursive check
	s.recursiveCheck = widget.NewCheck("Search subdirectories", nil)
	s.recursiveCheck.SetChecked(true)

	// Retry failed check
	s.retryFailedCheck = widget.NewCheck("Retry failed files", nil)

	// Mapping file (auto-set)
	s.mappingFileEntry = widget.NewEntry()
	s.mappingFileEntry.SetPlaceHolder("Auto-set to parent directory")

	mappingBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			s.mappingFileEntry.SetText(writer.URI().Path())
			writer.Close()
		}, s.window)
	})

	mappingRow := container.NewBorder(nil, nil, nil, mappingBrowseBtn, s.mappingFileEntry)

	// Build form
	content := container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabelWithStyle("Modality Types", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(s.metadataCheck, s.ultrasoundCheck),
			redactRow,
		),
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(s.recursiveCheck, s.retryFailedCheck),
		),
		widget.NewSeparator(),
		container.NewVBox(
			widget.NewLabelWithStyle("Mapping File", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Stores patient ID mappings for consistency"),
			mappingRow,
		),
	)

	return container.NewPadded(content)
}

// BuildStep3 creates the Preview step content
func (s *StepBuilder) BuildStep3() fyne.CanvasObject {
	// Title
	titleLabel := canvas.NewText("Preview (Dry Run)", ColorTextPrimary)
	titleLabel.TextSize = 18
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Progress
	s.previewProgress = widget.NewProgressBar()
	s.previewProgress.SetValue(0)

	s.previewStatus = widget.NewLabel("Scanning files...")

	// Results
	s.previewFilesList = widget.NewLabel("")
	s.previewFilesList.Wrapping = fyne.TextWrapWord

	s.previewPatients = widget.NewLabel("")
	s.previewPatients.Wrapping = fyne.TextWrapWord

	// Scrollable container for preview results
	previewScroll := container.NewVScroll(s.previewPatients)
	previewScroll.SetMinSize(fyne.NewSize(0, 200))

	// Container to update
	s.previewContainer = container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		s.previewProgress,
		s.previewStatus,
		widget.NewSeparator(),
		s.previewFilesList,
		widget.NewSeparator(),
	)

	// Use border layout to make scroll expand
	return container.NewBorder(
		container.NewPadded(s.previewContainer), // top
		nil, // bottom
		nil, // left
		nil, // right
		container.NewPadded(previewScroll), // center (fills remaining space)
	)
}

// BuildStep4 creates the Process step content
func (s *StepBuilder) BuildStep4() fyne.CanvasObject {
	// Title
	titleLabel := canvas.NewText("Processing", ColorTextPrimary)
	titleLabel.TextSize = 18
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Progress
	s.processProgress = widget.NewProgressBar()
	s.processProgress.SetValue(0)

	s.processStatus = widget.NewLabel("Ready to process")
	s.processFileCount = widget.NewLabel("")
	s.processCurrentFile = widget.NewLabel("")
	s.processCurrentFile.Wrapping = fyne.TextWrapWord

	s.processStats = widget.NewLabel("")
	s.processSummary = widget.NewLabel("")
	s.processSummary.Wrapping = fyne.TextWrapWord

	// Fixed header content (progress area)
	headerContent := container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		s.processProgress,
		s.processStatus,
		s.processFileCount,
		s.processCurrentFile,
		widget.NewSeparator(),
	)

	// Scrollable content (stats and summary that can grow)
	scrollableContent := container.NewVBox(
		s.processStats,
		s.processSummary,
	)
	processScroll := container.NewVScroll(scrollableContent)
	processScroll.SetMinSize(fyne.NewSize(0, 150))

	// Container to update (keep reference for compatibility)
	s.processContainer = container.NewVBox(
		headerContent,
		processScroll,
	)

	// Use border layout to make scroll expand
	return container.NewBorder(
		container.NewPadded(headerContent), // top (fixed)
		nil,                                // bottom
		nil,                                // left
		nil,                                // right
		container.NewPadded(processScroll), // center (fills remaining space, scrollable)
	)
}

// updateFileCount scans for DICOM files and updates the count label
func (s *StepBuilder) updateFileCount() {
	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	if inputFolder == "" {
		s.fileCountLabel.SetText("")
		return
	}

	s.fileCountLabel.SetText("Scanning...")

	go func() {
		files, err := dcm.FindDicomFiles(inputFolder, true)
		count := 0
		if err == nil {
			count = len(files)
		}

		// Update UI - Fyne v2.4 handles thread safety for widget updates
		if count == 0 {
			s.fileCountLabel.SetText("No DICOM files found")
		} else {
			s.fileCountLabel.SetText(fmt.Sprintf("Found %d DICOM file(s)", count))
		}
	}()
}

// autoSetMappingFile auto-sets the mapping file path based on input folder
func (s *StepBuilder) autoSetMappingFile() {
	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	if inputFolder == "" {
		return
	}

	parentDir := filepath.Dir(inputFolder)
	mappingPath := filepath.Join(parentDir, "patient_mapping.json")

	if s.mappingFileEntry != nil {
		s.mappingFileEntry.SetText(mappingPath)
	}
}

// ValidateStep1 validates the input step
func (s *StepBuilder) ValidateStep1() bool {
	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	if inputFolder == "" {
		dialog.ShowError(fmt.Errorf("please enter an input folder path"), s.window)
		return false
	}

	secretKey := strings.TrimSpace(s.secretKeyEntry.Text)
	if secretKey == "" {
		dialog.ShowError(fmt.Errorf("please enter a secret key or click 'Generate' to create one"), s.window)
		return false
	}

	return true
}

// ValidateStep2 validates the settings step
func (s *StepBuilder) ValidateStep2() bool {
	return true // Settings have defaults
}

// RunDryRun executes the dry run scan when entering step 3
func (s *StepBuilder) RunDryRun() {
	s.dryRunComplete = false
	s.dryRunStats = nil
	s.patientPreviewData = ""

	s.previewProgress.SetValue(0)
	s.previewStatus.SetText("Scanning files...")
	s.previewFilesList.SetText("")
	s.previewPatients.SetText("")
	s.wizard.SetNextEnabled(false)

	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	mappingFile := strings.TrimSpace(s.mappingFileEntry.Text)
	if mappingFile == "" {
		mappingFile = filepath.Join(filepath.Dir(inputFolder), "patient_mapping.json")
	}
	salt := s.secretKeyEntry.Text
	recursive := s.recursiveCheck.Checked

	go func() {
		// Find files
		s.previewStatus.SetText("Finding DICOM files...")
		s.previewProgress.SetValue(0.1)

		files, err := dcm.FindDicomFiles(inputFolder, recursive)
		if err != nil {
			s.previewStatus.SetText(fmt.Sprintf("Error: %v", err))
			return
		}

		if len(files) == 0 {
			s.previewStatus.SetText("No DICOM files found")
			s.previewFilesList.SetText("Please go back and check your input folder path.")
			return
		}

		s.previewStatus.SetText("Analyzing patient identities...")
		s.previewProgress.SetValue(0.3)
		s.previewFilesList.SetText(fmt.Sprintf("Found %d DICOM file(s)", len(files)))

		// Group files by patient
		mapper := identity.NewPseudonymizationMapper(mappingFile, salt)
		patients := groupFilesForPreview(files, salt)

		s.previewProgress.SetValue(0.7)

		// Generate preview of patient mappings
		var previewLines []string
		identityCount := 0
		pidCount := 0

		for _, patient := range patients {
			anonID, method := mapper.GetAnonID(patient.PID, patient.Name, patient.DOB)

			if method == identity.MatchIdentity {
				identityCount++
				previewLines = append(previewLines, fmt.Sprintf("  %s <- '%s' + DOB (%d files)",
					anonID, patient.Name, len(patient.Files)))
			} else {
				pidCount++
				previewLines = append(previewLines, fmt.Sprintf("  %s <- PID '%s' (%d files)",
					anonID, patient.PID, len(patient.Files)))
			}
		}

		s.patientPreviewData = strings.Join(previewLines, "\n")

		s.previewProgress.SetValue(1.0)
		s.previewStatus.SetText("Scan complete!")

		filesText := fmt.Sprintf("Files to process: %d\nUnique patients: %d", len(files), len(patients))
		s.previewFilesList.SetText(filesText)

		patientsText := fmt.Sprintf("Patient ID Mapping Preview:\n(Identity match: %d, PID match: %d)\n\n%s\n\nLooks good? Click \"Process\" to continue.",
			identityCount, pidCount, s.patientPreviewData)
		s.previewPatients.SetText(patientsText)

		s.dryRunComplete = true
		s.dryRunStats = &anonymizer.Stats{
			TotalPatients:   len(patients),
			IdentityMatched: identityCount,
			PIDMatched:      pidCount,
		}
		s.wizard.SetNextEnabled(true)
	}()
}

// PatientGroupPreview represents files grouped by patient for preview
type PatientGroupPreview struct {
	Key   string
	Name  string
	DOB   string
	PID   string
	Files []string
}

// groupFilesForPreview groups DICOM files by patient for preview
func groupFilesForPreview(files []string, salt string) []*PatientGroupPreview {
	patients := make(map[string]*PatientGroupPreview)

	for _, filePath := range files {
		ds, err := dcm.ReadDicomMetadataOnly(filePath)
		if err != nil {
			if patients["UNKNOWN"] == nil {
				patients["UNKNOWN"] = &PatientGroupPreview{Key: "UNKNOWN"}
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

		var key string
		if identity.IsValidIdentity(name, dob) {
			key = identity.CreateIdentityHash(name, dob, salt)
		} else {
			key = "PID:" + pid
		}

		if patients[key] == nil {
			patients[key] = &PatientGroupPreview{
				Key:  key,
				Name: name,
				DOB:  dob,
				PID:  pid,
			}
		}
		patients[key].Files = append(patients[key].Files, filePath)
	}

	result := make([]*PatientGroupPreview, 0, len(patients))
	for _, p := range patients {
		result = append(result, p)
	}

	return result
}

// RunProcess executes the actual anonymization
func (s *StepBuilder) RunProcess() {
	s.processingMu.Lock()
	if s.processing {
		s.processingMu.Unlock()
		return
	}
	s.processing = true
	s.processingMu.Unlock()

	s.processProgress.SetValue(0)
	s.processStatus.SetText("Starting...")
	s.processFileCount.SetText("")
	s.processCurrentFile.SetText("")
	s.processStats.SetText("")
	s.processSummary.SetText("")
	s.wizard.SetBackEnabled(false)
	s.wizard.SetNextEnabled(false)

	// Build config
	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	mappingFile := strings.TrimSpace(s.mappingFileEntry.Text)
	if mappingFile == "" {
		mappingFile = filepath.Join(filepath.Dir(inputFolder), "patient_mapping.json")
	}

	redactRows := 75
	if s.redactRowsEntry.Text != "" {
		if val, err := strconv.Atoi(s.redactRowsEntry.Text); err == nil && val > 0 {
			redactRows = val
		}
	}

	cfg := anonymizer.Config{
		InputFolder:       inputFolder,
		MappingFile:       mappingFile,
		Salt:              s.secretKeyEntry.Text,
		RedactRows:        redactRows,
		DryRun:            false,
		RetryFailed:       s.retryFailedCheck.Checked,
		Recursive:         s.recursiveCheck.Checked,
		ProcessMetadata:   s.metadataCheck.Checked,
		ProcessUltrasound: s.ultrasoundCheck.Checked,
		OutputWriter:      func(msg string) {}, // We use progress callback instead
	}

	go func() {
		defer func() {
			s.processingMu.Lock()
			s.processing = false
			s.processingMu.Unlock()
		}()

		// Progress callback
		successCount := 0
		failedCount := 0
		skippedCount := 0

		progressCallback := func(current, total int, filename, status string) {
			switch status {
			case "success":
				successCount++
			case "failed":
				failedCount++
			case "skipped":
				skippedCount++
			}

			// Update UI - Fyne v2.4 handles thread safety for widget updates
			progress := float64(current) / float64(total)
			s.processProgress.SetValue(progress)
			s.processFileCount.SetText(fmt.Sprintf("Processing %d/%d files", current, total))
			s.processCurrentFile.SetText(fmt.Sprintf("Current: %s", filename))
			s.processStats.SetText(fmt.Sprintf("Success: %d | Skipped: %d | Failed: %d",
				successCount, skippedCount, failedCount))
		}

		stats, err := anonymizer.ProcessFolderWithProgress(cfg, progressCallback)

		// Update UI with final state
		if err != nil {
			s.processStatus.SetText("Error!")
			s.processSummary.SetText(fmt.Sprintf("Error: %v", err))
		} else {
			s.processProgress.SetValue(1.0)
			s.processStatus.SetText("Complete!")
			s.processStats.SetText(fmt.Sprintf("Success: %d | Skipped: %d | Failed: %d",
				stats.Success, stats.Skipped, stats.Failed))
			s.processSummary.SetText(fmt.Sprintf(
				"Processed %d patient(s)\nIdentity matched: %d\nPatientID matched: %d\n\nOutput: %s/anonymized\nMapping: %s",
				stats.TotalPatients, stats.IdentityMatched, stats.PIDMatched,
				inputFolder, mappingFile))
		}

		s.wizard.SetNextText("Done")
		s.wizard.SetNextEnabled(true)
	}()
}

// GetConfig builds the anonymizer config from the current form values
func (s *StepBuilder) GetConfig() anonymizer.Config {
	inputFolder := strings.TrimSpace(s.inputFolderEntry.Text)
	mappingFile := strings.TrimSpace(s.mappingFileEntry.Text)
	if mappingFile == "" {
		mappingFile = filepath.Join(filepath.Dir(inputFolder), "patient_mapping.json")
	}

	redactRows := 75
	if s.redactRowsEntry.Text != "" {
		if val, err := strconv.Atoi(s.redactRowsEntry.Text); err == nil && val > 0 {
			redactRows = val
		}
	}

	return anonymizer.Config{
		InputFolder:       inputFolder,
		MappingFile:       mappingFile,
		Salt:              s.secretKeyEntry.Text,
		RedactRows:        redactRows,
		DryRun:            false,
		RetryFailed:       s.retryFailedCheck.Checked,
		Recursive:         s.recursiveCheck.Checked,
		ProcessMetadata:   s.metadataCheck.Checked,
		ProcessUltrasound: s.ultrasoundCheck.Checked,
	}
}

// IsProcessing returns whether processing is in progress
func (s *StepBuilder) IsProcessing() bool {
	s.processingMu.Lock()
	defer s.processingMu.Unlock()
	return s.processing
}
