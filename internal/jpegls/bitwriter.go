package jpegls

import "io"

// BitWriter provides bit-level writing to an underlying byte stream.
// Bits are written MSB-first (most significant bit first), which is
// the standard for JPEG-LS encoding.
type BitWriter struct {
	w        io.Writer
	buf      []byte // output buffer
	bitBuf   uint32 // current bits being accumulated
	bitCount int    // number of bits in bitBuf (0-32)
}

// NewBitWriter creates a new BitWriter that writes to w.
func NewBitWriter(w io.Writer) *BitWriter {
	return &BitWriter{
		w:   w,
		buf: make([]byte, 0, 4096),
	}
}

// WriteBit writes a single bit (0 or 1).
func (bw *BitWriter) WriteBit(bit int) {
	bw.bitBuf = (bw.bitBuf << 1) | uint32(bit&1)
	bw.bitCount++
	if bw.bitCount >= 8 {
		bw.flushByte()
	}
}

// WriteBits writes n bits from val (MSB first).
// The top n bits of val are ignored; only the low n bits are written.
func (bw *BitWriter) WriteBits(val, n int) {
	if n <= 0 {
		return
	}

	// Shift the bits into the buffer
	for i := n - 1; i >= 0; i-- {
		bit := (val >> i) & 1
		bw.WriteBit(bit)
	}
}

// WriteUnary writes a unary code: n zeros followed by a one.
func (bw *BitWriter) WriteUnary(n int) {
	for i := 0; i < n; i++ {
		bw.WriteBit(0)
	}
	bw.WriteBit(1)
}

// Write8 writes a full byte (8 bits).
func (bw *BitWriter) Write8(b byte) {
	bw.WriteBits(int(b), 8)
}

// Write16 writes a 16-bit value in big-endian order.
func (bw *BitWriter) Write16(val uint16) {
	bw.Write8(byte(val >> 8))
	bw.Write8(byte(val))
}

// WriteByteSlice writes multiple bytes.
func (bw *BitWriter) WriteByteSlice(data []byte) {
	for _, b := range data {
		bw.Write8(b)
	}
}

// flushByte writes one complete byte from the bit buffer.
func (bw *BitWriter) flushByte() {
	if bw.bitCount < 8 {
		return
	}

	// Extract the top 8 bits
	shift := bw.bitCount - 8
	b := byte(bw.bitBuf >> shift)
	bw.bitBuf &= (1 << shift) - 1
	bw.bitCount = shift

	bw.buf = append(bw.buf, b)

	// JPEG-LS requires byte stuffing after 0xFF
	// Insert a 0x00 byte after each 0xFF to prevent marker confusion
	if b == 0xFF {
		bw.buf = append(bw.buf, 0x00)
	}

	// Flush buffer if it gets large
	if len(bw.buf) >= 4000 {
		bw.flushBuffer()
	}
}

// flushBuffer writes the accumulated buffer to the underlying writer.
func (bw *BitWriter) flushBuffer() {
	if len(bw.buf) > 0 {
		bw.w.Write(bw.buf)
		bw.buf = bw.buf[:0]
	}
}

// Flush writes any remaining bits and the buffer to the output.
// Partial bytes are padded with 1 bits (per JPEG standard).
func (bw *BitWriter) Flush() error {
	// Pad with 1 bits to byte boundary (JPEG convention)
	for bw.bitCount > 0 && bw.bitCount < 8 {
		bw.WriteBit(1)
	}

	// Flush any remaining complete byte
	if bw.bitCount >= 8 {
		bw.flushByte()
	}

	// Write remaining buffer
	if len(bw.buf) > 0 {
		_, err := bw.w.Write(bw.buf)
		bw.buf = bw.buf[:0]
		return err
	}

	return nil
}

// ByteAlign pads the current byte with 1 bits and moves to the next byte.
func (bw *BitWriter) ByteAlign() {
	if bw.bitCount > 0 {
		for bw.bitCount < 8 {
			bw.WriteBit(1)
		}
		bw.flushByte()
	}
}

// WriteRaw writes raw bytes directly to the output without bit stuffing.
// This is used for writing JPEG markers.
// The bit buffer must be byte-aligned before calling this.
func (bw *BitWriter) WriteRaw(data []byte) error {
	// First flush the bit buffer
	bw.ByteAlign()
	bw.flushBuffer()

	// Write directly to output
	_, err := bw.w.Write(data)
	return err
}

// WriteMarker writes a JPEG marker (2 bytes: 0xFF followed by marker code).
func (bw *BitWriter) WriteMarker(marker byte) error {
	bw.ByteAlign()
	bw.flushBuffer()
	_, err := bw.w.Write([]byte{0xFF, marker})
	return err
}
