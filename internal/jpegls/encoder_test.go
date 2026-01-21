package jpegls

import (
	"bytes"
	"testing"
)

func TestPredict(t *testing.T) {
	tests := []struct {
		name     string
		a, b, c  int
		expected int
	}{
		{"horizontal edge", 100, 50, 150, 50}, // c >= max(a,b), predict min(a,b)
		{"vertical edge", 100, 150, 50, 150},  // c <= min(a,b), predict max(a,b)
		{"no edge", 100, 120, 110, 110},       // a + b - c = 100 + 120 - 110 = 110
		{"all equal", 100, 100, 100, 100},     // no edge: 100 + 100 - 100 = 100
		{"zero values", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Predict(tt.a, tt.b, tt.c)
			if result != tt.expected {
				t.Errorf("Predict(%d, %d, %d) = %d, want %d",
					tt.a, tt.b, tt.c, result, tt.expected)
			}
		})
	}
}

func TestQuantizeGradient(t *testing.T) {
	tests := []struct {
		g        int
		t1       int
		t2       int
		t3       int
		expected int
	}{
		{0, 3, 7, 21, 0},    // zero
		{1, 3, 7, 21, 1},    // small positive
		{-1, 3, 7, 21, -1},  // small negative
		{5, 3, 7, 21, 2},    // medium positive (between t1 and t2)
		{-5, 3, 7, 21, -2},  // medium negative
		{15, 3, 7, 21, 3},   // larger positive (between t2 and t3)
		{-15, 3, 7, 21, -3}, // larger negative
		{50, 3, 7, 21, 4},   // large positive (>= t3)
		{-50, 3, 7, 21, -4}, // large negative (<= -t3)
	}

	for _, tt := range tests {
		result := QuantizeGradient(tt.g, tt.t1, tt.t2, tt.t3)
		if result != tt.expected {
			t.Errorf("QuantizeGradient(%d, %d, %d, %d) = %d, want %d",
				tt.g, tt.t1, tt.t2, tt.t3, result, tt.expected)
		}
	}
}

func TestGetContextIndex(t *testing.T) {
	tests := []struct {
		q1, q2, q3 int
		wantIdx    int
		wantSign   int
	}{
		{0, 0, 0, 0*81 + 4*9 + 4, 1},   // all zero
		{1, 0, 0, 1*81 + 4*9 + 4, 1},   // positive q1
		{-1, 0, 0, 1*81 + 4*9 + 4, -1}, // negative q1 (sign flipped)
		{0, 1, 0, 0*81 + 5*9 + 4, 1},   // positive q2
		{0, -1, 0, 0*81 + 5*9 + 4, -1}, // negative q2 (sign flipped)
	}

	for _, tt := range tests {
		idx, sign := GetContextIndex(tt.q1, tt.q2, tt.q3)
		if idx != tt.wantIdx || sign != tt.wantSign {
			t.Errorf("GetContextIndex(%d, %d, %d) = (%d, %d), want (%d, %d)",
				tt.q1, tt.q2, tt.q3, idx, sign, tt.wantIdx, tt.wantSign)
		}
	}
}

func TestMapErrorValue(t *testing.T) {
	tests := []struct {
		errval   int
		near     int
		expected int
	}{
		{0, 0, 0},  // 0 -> 0
		{1, 0, 2},  // 1 -> 2
		{-1, 0, 1}, // -1 -> 1
		{2, 0, 4},  // 2 -> 4
		{-2, 0, 3}, // -2 -> 3
		{5, 0, 10}, // 5 -> 10
		{-5, 0, 9}, // -5 -> 9
	}

	for _, tt := range tests {
		result := MapErrorValue(tt.errval, tt.near)
		if result != tt.expected {
			t.Errorf("MapErrorValue(%d, %d) = %d, want %d",
				tt.errval, tt.near, result, tt.expected)
		}
	}
}

func TestUnmapErrorValue(t *testing.T) {
	tests := []struct {
		mapped   int
		expected int
	}{
		{0, 0},
		{2, 1},
		{1, -1},
		{4, 2},
		{3, -2},
	}

	for _, tt := range tests {
		result := UnmapErrorValue(tt.mapped)
		if result != tt.expected {
			t.Errorf("UnmapErrorValue(%d) = %d, want %d",
				tt.mapped, result, tt.expected)
		}
	}

	// Round-trip test
	for i := -100; i <= 100; i++ {
		mapped := MapErrorValue(i, 0)
		unmapped := UnmapErrorValue(mapped)
		if unmapped != i {
			t.Errorf("Round-trip failed: %d -> %d -> %d", i, mapped, unmapped)
		}
	}
}

func TestBitWriter(t *testing.T) {
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)

	// Write some bits
	bw.WriteBit(1)
	bw.WriteBit(0)
	bw.WriteBit(1)
	bw.WriteBit(0)
	bw.WriteBit(1)
	bw.WriteBit(0)
	bw.WriteBit(1)
	bw.WriteBit(0)

	bw.Flush()

	// Should produce 0b10101010 = 0xAA
	if buf.Len() != 1 || buf.Bytes()[0] != 0xAA {
		t.Errorf("BitWriter produced %x, want 0xAA", buf.Bytes())
	}
}

func TestBitWriterWriteBits(t *testing.T) {
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)

	// Write 0xFF (8 bits)
	bw.WriteBits(0xFF, 8)
	bw.Flush()

	// Should produce 0xFF followed by 0x00 (byte stuffing)
	expected := []byte{0xFF, 0x00}
	if !bytes.Equal(buf.Bytes(), expected) {
		t.Errorf("BitWriter produced %x, want %x", buf.Bytes(), expected)
	}
}

func TestBitWriterUnary(t *testing.T) {
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)

	// Unary(3) = 0001
	bw.WriteUnary(3)
	// Unary(0) = 1
	bw.WriteUnary(0)
	// Unary(2) = 001
	bw.WriteUnary(2)

	bw.Flush()

	// 0001 1 001 -> 00011001 = 0x19
	// With padding: 0001 1001 = 0x19
	if buf.Len() != 1 || buf.Bytes()[0] != 0x19 {
		t.Errorf("BitWriter produced %x, want 0x19", buf.Bytes())
	}
}

func TestContextComputeK(t *testing.T) {
	tests := []struct {
		a, n     int
		expected int
	}{
		{4, 1, 2},  // 2^2 = 4 >= 4/1
		{8, 2, 2},  // 2^2 = 4 >= 8/2
		{16, 1, 4}, // 2^4 = 16 >= 16/1
		{1, 1, 0},  // 2^0 = 1 >= 1/1
	}

	for _, tt := range tests {
		ctx := &Context{A: tt.a, N: tt.n}
		k := ctx.ComputeK(32)
		if k != tt.expected {
			t.Errorf("ComputeK(A=%d, N=%d) = %d, want %d",
				tt.a, tt.n, k, tt.expected)
		}
	}
}

func TestParams(t *testing.T) {
	// Test 8-bit parameters
	p8 := NewParams(8, 0)
	if p8.MaxVal != 255 {
		t.Errorf("8-bit MaxVal = %d, want 255", p8.MaxVal)
	}
	if p8.Range != 256 {
		t.Errorf("8-bit Range = %d, want 256", p8.Range)
	}

	// Test 12-bit parameters
	p12 := NewParams(12, 0)
	if p12.MaxVal != 4095 {
		t.Errorf("12-bit MaxVal = %d, want 4095", p12.MaxVal)
	}
	if p12.Range != 4096 {
		t.Errorf("12-bit Range = %d, want 4096", p12.Range)
	}

	// Test 16-bit parameters
	p16 := NewParams(16, 0)
	if p16.MaxVal != 65535 {
		t.Errorf("16-bit MaxVal = %d, want 65535", p16.MaxVal)
	}
}

func TestEncodeGrayscale(t *testing.T) {
	// Create a simple 4x4 test image
	pixels := []byte{
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
	}

	encoded, err := EncodeGrayscale(pixels, 4, 4)
	if err != nil {
		t.Fatalf("EncodeGrayscale failed: %v", err)
	}

	// Check that we got some output
	if len(encoded) == 0 {
		t.Error("EncodeGrayscale returned empty output")
	}

	// Check JPEG-LS markers
	if len(encoded) < 4 {
		t.Fatal("Output too short for markers")
	}
	if encoded[0] != 0xFF || encoded[1] != MarkerSOI {
		t.Errorf("Missing SOI marker, got %02x %02x", encoded[0], encoded[1])
	}
	if encoded[len(encoded)-2] != 0xFF || encoded[len(encoded)-1] != MarkerEOI {
		t.Errorf("Missing EOI marker, got %02x %02x",
			encoded[len(encoded)-2], encoded[len(encoded)-1])
	}
}

func TestEncodeGradientImage(t *testing.T) {
	// Create a gradient image (harder to compress)
	width, height := 16, 16
	pixels := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels[y*width+x] = byte((x + y) * 8)
		}
	}

	encoded, err := EncodeGrayscale(pixels, width, height)
	if err != nil {
		t.Fatalf("EncodeGrayscale failed: %v", err)
	}

	if len(encoded) == 0 {
		t.Error("EncodeGrayscale returned empty output")
	}

	t.Logf("Gradient image: %d bytes input, %d bytes output (%.1f%% ratio)",
		len(pixels), len(encoded), float64(len(encoded))*100/float64(len(pixels)))
}

func TestEncodeUniformImage(t *testing.T) {
	// Create a uniform image (should compress very well)
	width, height := 64, 64
	pixels := make([]byte, width*height)
	for i := range pixels {
		pixels[i] = 128
	}

	encoded, err := EncodeGrayscale(pixels, width, height)
	if err != nil {
		t.Fatalf("EncodeGrayscale failed: %v", err)
	}

	// Uniform images should compress significantly
	ratio := float64(len(encoded)) / float64(len(pixels))
	if ratio > 0.5 {
		t.Errorf("Uniform image compression ratio %.2f is too high (expected < 0.5)", ratio)
	}

	t.Logf("Uniform image: %d bytes input, %d bytes output (%.1f%% ratio)",
		len(pixels), len(encoded), ratio*100)
}

func TestEncode16Bit(t *testing.T) {
	// Create a 16-bit test image
	width, height := 8, 8
	pixels := make([]uint16, width*height)
	for i := range pixels {
		pixels[i] = uint16(i * 512) // 0 to 32768
	}

	encoded, err := EncodeGrayscale16(pixels, width, height, 16)
	if err != nil {
		t.Fatalf("EncodeGrayscale16 failed: %v", err)
	}

	if len(encoded) == 0 {
		t.Error("EncodeGrayscale16 returned empty output")
	}

	t.Logf("16-bit image: %d samples (%d bytes input), %d bytes output",
		len(pixels), len(pixels)*2, len(encoded))
}

func TestEncodeFromBytes(t *testing.T) {
	// Test 8-bit encoding
	data8 := []byte{100, 101, 102, 103, 100, 101, 102, 103}
	encoded8, err := EncodeFromBytes(data8, 4, 2, 1, 8)
	if err != nil {
		t.Fatalf("EncodeFromBytes (8-bit) failed: %v", err)
	}
	if len(encoded8) == 0 {
		t.Error("EncodeFromBytes (8-bit) returned empty output")
	}

	// Test 16-bit encoding (little-endian)
	data16 := []byte{
		0x00, 0x01, // 256
		0x01, 0x01, // 257
		0x02, 0x01, // 258
		0x03, 0x01, // 259
	}
	encoded16, err := EncodeFromBytes(data16, 2, 2, 1, 16)
	if err != nil {
		t.Fatalf("EncodeFromBytes (16-bit) failed: %v", err)
	}
	if len(encoded16) == 0 {
		t.Error("EncodeFromBytes (16-bit) returned empty output")
	}
}

func TestNeighborGetter(t *testing.T) {
	// Create a 4x4 test image
	pixels := []int{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}

	ng := NewNeighborGetter(pixels, 4, 4, 128)

	// Test interior pixel (2, 2) -> value 11
	// Neighbors: a=10, b=7, c=6, d=8
	a, b, c, d := ng.GetNeighbors(2, 2)
	if a != 10 || b != 7 || c != 6 || d != 8 {
		t.Errorf("GetNeighbors(2,2) = (%d,%d,%d,%d), want (10,7,6,8)", a, b, c, d)
	}

	// Test first row
	a, b, c, d = ng.GetNeighbors(1, 0)
	if b != 128 || c != 128 || d != 128 {
		t.Errorf("GetNeighbors(1,0) upper neighbors should be 128, got b=%d c=%d d=%d", b, c, d)
	}

	// Test first column
	a, b, c, d = ng.GetNeighbors(0, 2)
	if a != b || c != b {
		t.Errorf("GetNeighbors(0,2) first column: a=%d b=%d c=%d (a and c should equal b)", a, b, c)
	}
}

func TestRunModeDetection(t *testing.T) {
	if !DetectRunMode(0, 0, 0) {
		t.Error("DetectRunMode(0,0,0) should return true")
	}
	if DetectRunMode(1, 0, 0) {
		t.Error("DetectRunMode(1,0,0) should return false")
	}
	if DetectRunMode(0, 1, 0) {
		t.Error("DetectRunMode(0,1,0) should return false")
	}
}

func BenchmarkEncodeGrayscale(b *testing.B) {
	// Create a 256x256 test image
	width, height := 256, 256
	pixels := make([]byte, width*height)
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeGrayscale(pixels, width, height)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode16Bit(b *testing.B) {
	// Create a 256x256 16-bit test image
	width, height := 256, 256
	pixels := make([]uint16, width*height)
	for i := range pixels {
		pixels[i] = uint16(i % 65536)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeGrayscale16(pixels, width, height, 16)
		if err != nil {
			b.Fatal(err)
		}
	}
}
