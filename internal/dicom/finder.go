package dicom

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/suyashkumar/dicom"
)

// DicomExtensions are common DICOM file extensions
var DicomExtensions = []string{".dcm", ".DCM", ".dicom", ".DICOM"}

// ExcludedNames are filenames to skip
var ExcludedNames = map[string]bool{
	"DICOMDIR":        true,
	".progress.json":  true,
	".DS_Store":       true,
	"Thumbs.db":       true,
	"desktop.ini":     true,
	"Makefile":        true,
	"README":          true,
	"README.md":       true,
	"LICENSE":         true,
	"CHANGELOG":       true,
	"CHANGELOG.md":    true,
	".gitignore":      true,
	".git":            true,
	"go.mod":          true,
	"go.sum":          true,
	"package.json":    true,
	"package-lock.json": true,
}

// ExcludedExtensions are file extensions to skip
var ExcludedExtensions = map[string]bool{
	".go":     true,
	".py":     true,
	".js":     true,
	".ts":     true,
	".json":   true,
	".yaml":   true,
	".yml":    true,
	".xml":    true,
	".txt":    true,
	".md":     true,
	".log":    true,
	".csv":    true,
	".exe":    true,
	".dll":    true,
	".so":     true,
	".dylib":  true,
	".app":    true,
	".zip":    true,
	".tar":    true,
	".gz":     true,
	".rar":    true,
	".7z":     true,
	".png":    true,
	".jpg":    true,
	".jpeg":   true,
	".gif":    true,
	".bmp":    true,
	".pdf":    true,
	".doc":    true,
	".docx":   true,
	".xls":    true,
	".xlsx":   true,
	".ppt":    true,
	".pptx":   true,
	".html":   true,
	".htm":    true,
	".css":    true,
	".sh":     true,
	".bat":    true,
	".ps1":    true,
	".toml":   true,
	".lock":   true,
	".sum":    true,
	".mod":    true,
}

// ExcludedDirs are directory names to skip entirely
var ExcludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"build":        true,
	"dist":         true,
	"bin":          true,
	"obj":          true,
	"target":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".idea":        true,
	".vscode":      true,
}

// FindDicomFiles finds all DICOM files in the given path.
func FindDicomFiles(inputPath string, recursive bool) ([]string, error) {
	var files []string
	seenFiles := make(map[string]bool)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			// Skip excluded directories
			if ExcludedDirs[info.Name()] {
				return filepath.SkipDir
			}
			// If not recursive and this is a subdirectory, skip it
			if !recursive && path != inputPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded filenames
		if ExcludedNames[info.Name()] {
			return nil
		}

		// Skip files in "anonymized" directories
		if strings.Contains(path, "anonymized") {
			return nil
		}

		// Check extension - skip known non-DICOM extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ExcludedExtensions[ext] {
			return nil
		}

		// Check for DICOM extension
		isDicom := false
		for _, de := range DicomExtensions {
			if ext == strings.ToLower(de) {
				isDicom = true
				break
			}
		}

		// If no recognized extension, check DICOM magic bytes first
		if !isDicom && (ext == "" || !ExcludedExtensions[ext]) {
			if hasDicomMagicBytes(path) {
				isDicom = true
			}
		}

		if isDicom && !seenFiles[path] {
			files = append(files, path)
			seenFiles[path] = true
		}

		return nil
	}

	if err := filepath.Walk(inputPath, walkFn); err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// hasDicomMagicBytes checks if a file has the DICOM magic bytes ("DICM" at offset 128)
func hasDicomMagicBytes(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// DICOM files have "DICM" at byte offset 128
	header := make([]byte, 132)
	n, err := io.ReadFull(file, header)
	if err != nil || n < 132 {
		return false
	}

	// Check for "DICM" magic bytes at offset 128
	return string(header[128:132]) == "DICM"
}

// isDicomFile checks if a file is a valid DICOM file by trying to parse it
func isDicomFile(path string) bool {
	// First check magic bytes (fast check)
	if !hasDicomMagicBytes(path) {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Try to parse just the metadata
	_, err = dicom.Parse(file, 0, nil, dicom.SkipPixelData())
	return err == nil
}
