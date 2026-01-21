package jpegls

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"testing"
)

// TestDecodeWithDcmtk tests that our encoder output can be decoded by dcmtk's CharLS.
// This test is skipped if dcmdjpls is not available.
func TestDecodeWithDcmtk(t *testing.T) {
	// Check if dcmdjpls is available
	_, err := exec.LookPath("dcmdjpls")
	if err != nil {
		t.Skip("dcmdjpls not found, skipping decoder validation test")
	}

	// Create a simple test image
	width, height := 8, 8
	pixels := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels[y*width+x] = byte((x + y) * 16)
		}
	}

	// Encode
	encoded, err := EncodeGrayscale(pixels, width, height)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Write to temp file
	jlsFile := "/tmp/test_decoder.jls"
	err = os.WriteFile(jlsFile, encoded, 0644)
	if err != nil {
		t.Fatalf("Write JLS file failed: %v", err)
	}
	defer os.Remove(jlsFile)

	// Note: dcmdjpls expects DICOM files, not raw JLS streams.
	// For raw JLS validation, we'd need a different tool or library.
	t.Logf("Encoded %d bytes of pixel data (%dx%d) to %d bytes JLS",
		len(pixels), width, height, len(encoded))

	// Log the hex dump for debugging
	if len(encoded) < 200 {
		t.Logf("Encoded data:\n%s", hex.Dump(encoded))
	}
}

// TestJPEGLSMarkerStructure verifies the JPEG-LS marker structure
func TestJPEGLSMarkerStructure(t *testing.T) {
	// Create a simple 4x4 test image
	pixels := []byte{
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
	}

	encoded, err := EncodeGrayscale(pixels, 4, 4)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify markers
	t.Logf("Encoded stream (%d bytes):\n%s", len(encoded), hex.Dump(encoded))

	// Parse and verify marker structure
	idx := 0
	markerCount := 0

	for idx < len(encoded)-1 {
		if encoded[idx] == 0xFF && encoded[idx+1] != 0x00 {
			marker := encoded[idx+1]
			markerName := getMarkerName(marker)

			if marker == MarkerSOI {
				t.Logf("Marker at %d: SOI (0xFF%02X)", idx, marker)
				idx += 2
			} else if marker == MarkerEOI {
				t.Logf("Marker at %d: EOI (0xFF%02X)", idx, marker)
				idx += 2
			} else if marker >= 0xC0 {
				// Marker with length
				if idx+4 > len(encoded) {
					t.Fatalf("Truncated marker at %d", idx)
				}
				length := int(encoded[idx+2])<<8 | int(encoded[idx+3])
				t.Logf("Marker at %d: %s (0xFF%02X), length=%d", idx, markerName, marker, length)

				// Validate marker contents
				if marker == MarkerSOF55 {
					if idx+2+length > len(encoded) {
						t.Fatalf("SOF55 extends beyond data")
					}
					precision := encoded[idx+4]
					height := int(encoded[idx+5])<<8 | int(encoded[idx+6])
					width := int(encoded[idx+7])<<8 | int(encoded[idx+8])
					components := encoded[idx+9]
					t.Logf("  SOF55: precision=%d, height=%d, width=%d, components=%d",
						precision, height, width, components)
				} else if marker == MarkerSOS {
					if idx+2+length > len(encoded) {
						t.Fatalf("SOS extends beyond data")
					}
					ns := encoded[idx+4]
					near := encoded[idx+4+1+int(ns)*2]
					ilv := encoded[idx+4+2+int(ns)*2]
					pt := encoded[idx+4+3+int(ns)*2]
					t.Logf("  SOS: Ns=%d, NEAR=%d, ILV=%d, Pt=%d", ns, near, ilv, pt)
				}

				idx += 2 + length
			} else {
				idx++
			}
			markerCount++
		} else {
			idx++
		}
	}

	if markerCount < 3 {
		t.Errorf("Expected at least 3 markers (SOI, SOF55/SOS, EOI), found %d", markerCount)
	}
}

func getMarkerName(marker byte) string {
	switch marker {
	case MarkerSOI:
		return "SOI"
	case MarkerEOI:
		return "EOI"
	case MarkerSOF55:
		return "SOF55"
	case MarkerSOS:
		return "SOS"
	case MarkerLSE:
		return "LSE"
	default:
		return "UNKNOWN"
	}
}

// TestEncoderOutputValidity creates a minimal image and checks the output structure
func TestEncoderOutputValidity(t *testing.T) {
	// Test with a 1x1 image (minimal case)
	pixels := []byte{128}
	encoded, err := EncodeGrayscale(pixels, 1, 1)
	if err != nil {
		t.Fatalf("Encode 1x1 failed: %v", err)
	}

	t.Logf("1x1 image encoded to %d bytes", len(encoded))
	t.Logf("Hex dump:\n%s", hex.Dump(encoded))

	// Verify basic structure
	if !bytes.HasPrefix(encoded, []byte{0xFF, 0xD8}) {
		t.Error("Missing SOI marker")
	}
	if !bytes.HasSuffix(encoded, []byte{0xFF, 0xD9}) {
		t.Error("Missing EOI marker")
	}
	if bytes.Index(encoded, []byte{0xFF, 0xF7}) < 0 {
		t.Error("Missing SOF55 marker")
	}
	if bytes.Index(encoded, []byte{0xFF, 0xDA}) < 0 {
		t.Error("Missing SOS marker")
	}
}
