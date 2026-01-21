package dicom

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"dicom-anonymizer/internal/jpegls"
)

// JPEG-LS Transfer Syntax UIDs
const (
	JPEGLSLossless   = "1.2.840.10008.1.2.4.80"
	JPEGLSNearLossy  = "1.2.840.10008.1.2.4.81"

	// Explicit VR Little Endian (uncompressed)
	ExplicitVRLittleEndian = "1.2.840.10008.1.2.1"
)

// IsJPEGLSCompressed checks if a DICOM file uses JPEG-LS compression.
func IsJPEGLSCompressed(path string) bool {
	ds, err := ReadDicomMetadataOnly(path)
	if err != nil {
		return false
	}

	ts := ds.GetTransferSyntax()
	return strings.Contains(ts, JPEGLSLossless) || strings.Contains(ts, JPEGLSNearLossy)
}

// DecompressJPEGLS decompresses a JPEG-LS DICOM file using dcmtk.
// Returns the path to the decompressed temporary file.
func DecompressJPEGLS(inputPath string) (string, error) {
	// Check if dcmdjpls is available
	_, err := exec.LookPath("dcmdjpls")
	if err != nil {
		return "", fmt.Errorf("dcmtk not installed. Run: brew install dcmtk (macOS) or apt install dcmtk (Linux)")
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "dicom-*.dcm")
	if err != nil {
		return "", fmt.Errorf("could not create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Run dcmdjpls to decompress
	cmd := exec.Command("dcmdjpls", inputPath, tempPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("dcmdjpls failed: %s", string(output))
	}

	return tempPath, nil
}

// CheckDcmtkInstalled checks if dcmtk is installed.
// It checks both PATH and common installation directories.
func CheckDcmtkInstalled() bool {
	// First try standard PATH lookup
	_, err := exec.LookPath("dcmdjpls")
	if err == nil {
		return true
	}

	// Check common installation paths that might not be in PATH
	var commonPaths []string
	switch runtime.GOOS {
	case "darwin":
		// Homebrew paths (ARM and Intel)
		commonPaths = []string{
			"/opt/homebrew/bin/dcmdjpls",
			"/usr/local/bin/dcmdjpls",
		}
	case "linux":
		commonPaths = []string{
			"/usr/bin/dcmdjpls",
			"/usr/local/bin/dcmdjpls",
		}
	case "windows":
		commonPaths = []string{
			"C:\\Program Files\\dcmtk\\bin\\dcmdjpls.exe",
			"C:\\dcmtk\\bin\\dcmdjpls.exe",
		}
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// CompressJPEGLS compresses pixel data using JPEG-LS lossless compression.
// This is a pure Go implementation that doesn't require external dependencies.
//
// Parameters:
//   - pixels: raw pixel data in row-major order
//   - width, height: image dimensions
//   - samples: samples per pixel (1 for grayscale, 3 for RGB)
//   - bitsAllocated: bits allocated per sample (8, 12, or 16)
//
// Returns the JPEG-LS compressed bitstream.
func CompressJPEGLS(pixels []byte, width, height, samples, bitsAllocated int) ([]byte, error) {
	return jpegls.EncodeFromBytes(pixels, width, height, samples, bitsAllocated)
}

// CompressJPEGLSMultiFrame compresses multiple frames using JPEG-LS and returns
// encapsulated pixel data suitable for DICOM.
func CompressJPEGLSMultiFrame(frames [][]byte, width, height, samples, bitsAllocated int) ([]byte, error) {
	compressedFrames := make([][]byte, len(frames))

	for i, frame := range frames {
		compressed, err := CompressJPEGLS(frame, width, height, samples, bitsAllocated)
		if err != nil {
			return nil, fmt.Errorf("failed to compress frame %d: %w", i, err)
		}
		compressedFrames[i] = compressed
	}

	return EncapsulateFrames(compressedFrames), nil
}
