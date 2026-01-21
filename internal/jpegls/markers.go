package jpegls

import (
	"bytes"
	"encoding/binary"
)

// JPEG-LS Marker codes
const (
	// Start of image
	MarkerSOI = 0xD8

	// End of image
	MarkerEOI = 0xD9

	// Start of frame (JPEG-LS)
	MarkerSOF55 = 0xF7

	// Start of scan (JPEG-LS)
	MarkerSOS = 0xDA

	// JPEG-LS preset parameters
	MarkerLSE = 0xF8

	// Application segments
	MarkerAPP0 = 0xE0
	MarkerAPP8 = 0xE8

	// Comment
	MarkerCOM = 0xFE
)

// LSE types (preset parameter types)
const (
	LSEPresetParams = 1 // Preset coding parameters
	LSEMappingTable = 2 // Mapping table specification
)

// FrameInfo contains the information needed for frame header.
type FrameInfo struct {
	Width          int
	Height         int
	BitsPerSample  int
	ComponentCount int
}

// ScanInfo contains the information needed for scan header.
type ScanInfo struct {
	Near      int  // NEAR parameter (0 = lossless)
	ILV       int  // Interleave mode (0 = none, 1 = line, 2 = sample)
	Pt        int  // Point transform (always 0 for us)
	MaxVal    int  // Maximum sample value
	T1        int  // Threshold 1
	T2        int  // Threshold 2
	T3        int  // Threshold 3
	Reset     int  // Reset interval
	UsePreset bool // Whether to include LSE preset marker
}

// WriteSOI writes the Start of Image marker.
func WriteSOI(buf *bytes.Buffer) {
	buf.Write([]byte{0xFF, MarkerSOI})
}

// WriteEOI writes the End of Image marker.
func WriteEOI(buf *bytes.Buffer) {
	buf.Write([]byte{0xFF, MarkerEOI})
}

// WriteSOF55 writes the JPEG-LS Start of Frame marker segment.
// This identifies the image as JPEG-LS encoded.
//
// Structure:
//   - Marker: 0xFF 0xF7
//   - Length: 2 bytes (8 + 3*Nf)
//   - P: 1 byte (sample precision)
//   - Y: 2 bytes (height)
//   - X: 2 bytes (width)
//   - Nf: 1 byte (number of components)
//   - For each component:
//   - Ci: 1 byte (component ID)
//   - Hi:Vi: 1 byte (sampling factors, always 0x11)
//   - Tqi: 1 byte (quantization table, always 0)
func WriteSOF55(buf *bytes.Buffer, info FrameInfo) {
	// Calculate segment length
	nf := info.ComponentCount
	if nf == 0 {
		nf = 1
	}
	length := 8 + 3*nf

	// Write marker
	buf.Write([]byte{0xFF, MarkerSOF55})

	// Write length (big-endian)
	binary.Write(buf, binary.BigEndian, uint16(length))

	// Sample precision (P)
	buf.WriteByte(byte(info.BitsPerSample))

	// Height (Y)
	binary.Write(buf, binary.BigEndian, uint16(info.Height))

	// Width (X)
	binary.Write(buf, binary.BigEndian, uint16(info.Width))

	// Number of components (Nf)
	buf.WriteByte(byte(nf))

	// Component specifications
	for i := 0; i < nf; i++ {
		// Component ID (Ci) - use 1-based index
		buf.WriteByte(byte(i + 1))
		// Sampling factors (Hi:Vi) - always 1:1 for JPEG-LS
		buf.WriteByte(0x11)
		// Quantization table (Tqi) - not used in JPEG-LS
		buf.WriteByte(0)
	}
}

// WriteSOS writes the JPEG-LS Start of Scan marker segment.
//
// Structure:
//   - Marker: 0xFF 0xDA
//   - Length: 2 bytes (6 + 2*Ns)
//   - Ns: 1 byte (number of components in scan)
//   - For each component:
//   - Csj: 1 byte (component selector)
//   - Tdj:Taj: 1 byte (table selectors, always 0)
//   - NEAR: 1 byte (loss parameter)
//   - ILV: 1 byte (interleave mode)
//   - Pt: 1 byte (point transform)
func WriteSOS(buf *bytes.Buffer, info ScanInfo, componentCount int) {
	if componentCount <= 0 {
		componentCount = 1
	}
	components := make([]int, componentCount)
	for i := 0; i < componentCount; i++ {
		components[i] = i + 1
	}
	WriteSOSComponents(buf, info, components)
}

// WriteSOSComponents writes the JPEG-LS Start of Scan marker for explicit component IDs.
// componentIDs should use 1-based component identifiers.
func WriteSOSComponents(buf *bytes.Buffer, info ScanInfo, componentIDs []int) {
	ns := len(componentIDs)
	if ns == 0 {
		ns = 1
		componentIDs = []int{1}
	}
	length := 6 + 2*ns

	// Write marker
	buf.Write([]byte{0xFF, MarkerSOS})

	// Write length
	binary.Write(buf, binary.BigEndian, uint16(length))

	// Number of components in scan (Ns)
	buf.WriteByte(byte(ns))

	// Component specifications
	for _, compID := range componentIDs {
		// Component selector (Csj)
		buf.WriteByte(byte(compID))
		// Table selectors (not used in JPEG-LS)
		buf.WriteByte(0)
	}

	// NEAR parameter
	buf.WriteByte(byte(info.Near))

	// Interleave mode (ILV)
	buf.WriteByte(byte(info.ILV))

	// Point transform (Pt) - always 0 for us
	buf.WriteByte(byte(info.Pt))
}

// WriteLSEPreset writes the JPEG-LS preset parameters marker.
// This is optional but helps decoders know the exact parameters used.
//
// Structure:
//   - Marker: 0xFF 0xF8
//   - Length: 2 bytes (13)
//   - ID: 1 byte (1 = preset params)
//   - MaxVal: 2 bytes
//   - T1: 2 bytes
//   - T2: 2 bytes
//   - T3: 2 bytes
//   - Reset: 2 bytes
func WriteLSEPreset(buf *bytes.Buffer, info ScanInfo) {
	// Write marker
	buf.Write([]byte{0xFF, MarkerLSE})

	// Length = 13 bytes (includes length field)
	binary.Write(buf, binary.BigEndian, uint16(13))

	// ID = 1 (preset coding parameters)
	buf.WriteByte(LSEPresetParams)

	// MAXVAL
	binary.Write(buf, binary.BigEndian, uint16(info.MaxVal))

	// T1
	binary.Write(buf, binary.BigEndian, uint16(info.T1))

	// T2
	binary.Write(buf, binary.BigEndian, uint16(info.T2))

	// T3
	binary.Write(buf, binary.BigEndian, uint16(info.T3))

	// RESET
	binary.Write(buf, binary.BigEndian, uint16(info.Reset))
}

// WriteJPEGLSHeader writes the complete JPEG-LS header (SOI, SOF55, optionally LSE, SOS).
func WriteJPEGLSHeader(buf *bytes.Buffer, frame FrameInfo, scan ScanInfo) {
	WriteSOI(buf)
	WriteSOF55(buf, frame)
	if scan.UsePreset {
		WriteLSEPreset(buf, scan)
	}
	WriteSOS(buf, scan, frame.ComponentCount)
}

// WriteJPEGLSTrailer writes the JPEG-LS trailer (EOI marker).
func WriteJPEGLSTrailer(buf *bytes.Buffer) {
	WriteEOI(buf)
}
