package dicom

import (
	"bytes"
	"encoding/binary"
)

// DICOM encapsulated pixel data tags
const (
	// Item tag (FFFE,E000)
	ItemTag uint32 = 0xFFFEE000
	// Item delimitation tag (FFFE,E00D)
	ItemDelimTag uint32 = 0xFFFEE00D
	// Sequence delimitation tag (FFFE,E0DD)
	SeqDelimTag uint32 = 0xFFFEE0DD
)

// EncapsulateFrames creates encapsulated pixel data from compressed frames.
// This is the format required for DICOM files with compressed transfer syntaxes.
//
// The structure is:
// 1. Basic Offset Table (Item tag + length + offsets)
// 2. Frame data items (Item tag + length + data for each frame)
// 3. Sequence delimitation item
//
// All values are little-endian as per DICOM standard.
func EncapsulateFrames(frames [][]byte) []byte {
	var buf bytes.Buffer

	// Calculate offsets for the Basic Offset Table
	offsets := make([]uint32, len(frames))
	currentOffset := uint32(0)
	for i, frame := range frames {
		offsets[i] = currentOffset
		// Each frame item: 4 bytes tag + 4 bytes length + frame data (padded to even)
		frameLen := uint32(len(frame))
		if frameLen%2 != 0 {
			frameLen++ // padding
		}
		currentOffset += 8 + frameLen
	}

	// Write Basic Offset Table
	writeBasicOffsetTable(&buf, offsets)

	// Write each frame as an item
	for _, frame := range frames {
		writeFrameItem(&buf, frame)
	}

	// Write sequence delimitation item
	writeSequenceDelimiter(&buf)

	return buf.Bytes()
}

// EncapsulateSingleFrame is a convenience function for single-frame images.
func EncapsulateSingleFrame(frameData []byte) []byte {
	return EncapsulateFrames([][]byte{frameData})
}

// writeBasicOffsetTable writes the Basic Offset Table item.
// For single-frame images, the BOT can be empty (zero length).
// For multi-frame images, it contains byte offsets to each frame.
func writeBasicOffsetTable(buf *bytes.Buffer, offsets []uint32) {
	// Item tag (little-endian)
	binary.Write(buf, binary.LittleEndian, uint16(0xFFFE))
	binary.Write(buf, binary.LittleEndian, uint16(0xE000))

	// Length of offset table
	if len(offsets) <= 1 {
		// Single frame or empty: BOT can be empty
		binary.Write(buf, binary.LittleEndian, uint32(0))
	} else {
		// Multi-frame: write all offsets
		binary.Write(buf, binary.LittleEndian, uint32(len(offsets)*4))
		for _, offset := range offsets {
			binary.Write(buf, binary.LittleEndian, offset)
		}
	}
}

// writeFrameItem writes a single frame as a DICOM item.
func writeFrameItem(buf *bytes.Buffer, data []byte) {
	// Item tag
	binary.Write(buf, binary.LittleEndian, uint16(0xFFFE))
	binary.Write(buf, binary.LittleEndian, uint16(0xE000))

	// Length (must be even)
	length := uint32(len(data))
	if length%2 != 0 {
		length++ // will pad with zero
	}
	binary.Write(buf, binary.LittleEndian, length)

	// Frame data
	buf.Write(data)

	// Pad to even length if necessary
	if len(data)%2 != 0 {
		buf.WriteByte(0)
	}
}

// writeSequenceDelimiter writes the sequence delimitation item.
func writeSequenceDelimiter(buf *bytes.Buffer) {
	// Sequence delimitation tag
	binary.Write(buf, binary.LittleEndian, uint16(0xFFFE))
	binary.Write(buf, binary.LittleEndian, uint16(0xE0DD))

	// Length is always 0 for delimiter
	binary.Write(buf, binary.LittleEndian, uint32(0))
}

// ExtractFramesFromEncapsulated extracts individual frames from encapsulated pixel data.
// This is the inverse of EncapsulateFrames.
func ExtractFramesFromEncapsulated(data []byte) ([][]byte, error) {
	var frames [][]byte
	offset := 0

	// Skip Basic Offset Table
	if len(data) < 8 {
		return nil, nil
	}

	// Check for item tag
	if data[0] == 0xFE && data[1] == 0xFF && data[2] == 0x00 && data[3] == 0xE0 {
		// Read BOT length
		botLen := binary.LittleEndian.Uint32(data[4:8])
		offset = 8 + int(botLen)
	}

	// Read frame items
	for offset < len(data)-8 {
		// Check for sequence delimiter
		if data[offset] == 0xFE && data[offset+1] == 0xFF &&
			data[offset+2] == 0xDD && data[offset+3] == 0xE0 {
			break
		}

		// Check for item tag
		if data[offset] != 0xFE || data[offset+1] != 0xFF ||
			data[offset+2] != 0x00 || data[offset+3] != 0xE0 {
			break
		}

		// Read item length
		itemLen := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		// Extract frame data
		if offset+int(itemLen) <= len(data) {
			frame := make([]byte, itemLen)
			copy(frame, data[offset:offset+int(itemLen)])
			frames = append(frames, frame)
		}
		offset += int(itemLen)
	}

	return frames, nil
}
