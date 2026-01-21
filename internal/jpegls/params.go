// Package jpegls implements a pure Go JPEG-LS encoder according to ITU-T T.87.
package jpegls

// Default threshold values from ITU-T T.87 Table A.1
// These are for NEAR=0 (lossless) and MAXVAL=255 (8-bit samples)
const (
	// DefaultT1 is the first quantization threshold
	DefaultT1 = 3
	// DefaultT2 is the second quantization threshold
	DefaultT2 = 7
	// DefaultT3 is the third quantization threshold
	DefaultT3 = 21

	// DefaultReset is the context reset threshold (value of N at reset)
	DefaultReset = 64

	// ContextCount is the number of regular contexts (365 per ITU-T T.87)
	ContextCount = 365

	// RunContextCount is the number of run mode contexts
	RunContextCount = 2

	// MaxContextIndex is the maximum valid context index
	MaxContextIndex = ContextCount - 1

	// GradientRange is the range of quantized gradient values [-4, 4]
	GradientRange = 9

	// MinC is the minimum bias correction value
	MinC = -128
	// MaxC is the maximum bias correction value
	MaxC = 127

	// InitA is the initial value for A (accumulated prediction errors)
	InitA = 4

	// InitN is the initial value for N (context occurrence count)
	InitN = 1

	// LimitK is the maximum k parameter for Golomb coding
	LimitK = 32

	// Limit is the limit for unary code length (used in limited-length Golomb)
	Limit = 32
)

// JTable contains the run length coding order table from ITU-T T.87 Table A.2
// These values determine the number of bits used to encode run lengths
var JTable = []struct {
	RK int // Number of bits to use
	RN int // Run interruption sample count
}{
	{0, 0}, {0, 0}, {0, 0}, {0, 0},
	{1, 1}, {1, 1}, {1, 1}, {1, 1},
	{2, 2}, {2, 2}, {2, 2}, {2, 2},
	{3, 3}, {3, 3}, {3, 3}, {3, 3},
	{4, 4}, {4, 4}, {5, 5}, {5, 5},
	{6, 6}, {7, 7}, {8, 8}, {9, 9},
	{10, 10}, {11, 11}, {12, 12}, {13, 13},
	{14, 14}, {15, 15}, {16, 16}, {17, 17},
}

// Params holds the encoding parameters
type Params struct {
	// MAXVAL is the maximum sample value (2^bpp - 1)
	MaxVal int

	// NEAR is the loss parameter (0 for lossless)
	Near int

	// T1, T2, T3 are quantization thresholds
	T1, T2, T3 int

	// Reset is the context reset threshold
	Reset int

	// qbpp is the number of bits needed for a mapped error value
	Qbpp int

	// RANGE is (MAXVAL + 2*NEAR) / (2*NEAR + 1) + 1 for lossless
	Range int

	// LIMIT is the limit for Golomb coding
	Limit int

	// Bits per pixel
	BitsPerPixel int
}

// NewParams creates encoding parameters for the given bit depth and NEAR value
func NewParams(bpp, near int) *Params {
	maxVal := (1 << bpp) - 1

	// Calculate RANGE (A.2.1 in ITU-T T.87)
	var rangeVal int
	if near == 0 {
		rangeVal = maxVal + 1
	} else {
		rangeVal = (maxVal + 2*near) / (2*near + 1) + 1
	}

	// Calculate thresholds based on MAXVAL (Table A.1 in ITU-T T.87)
	t1, t2, t3 := calculateThresholds(maxVal, near)

	// qbpp is ceil(log2(RANGE))
	qbpp := 0
	for (1 << qbpp) < rangeVal {
		qbpp++
	}

	// LIMIT (A.7 in ITU-T T.87)
	limit := 2 * (bpp + max(8, bpp))

	return &Params{
		MaxVal:       maxVal,
		Near:         near,
		T1:           t1,
		T2:           t2,
		T3:           t3,
		Reset:        DefaultReset,
		Qbpp:         qbpp,
		Range:        rangeVal,
		Limit:        limit,
		BitsPerPixel: bpp,
	}
}

// calculateThresholds computes T1, T2, T3 based on MAXVAL and NEAR
// This follows ITU-T T.87 Annex A.1
func calculateThresholds(maxVal, near int) (t1, t2, t3 int) {
	if maxVal >= 128 {
		factor := maxVal
		if factor > 4095 {
			factor = 4095
		}
		t1 = clamp(near+1+factor/256, near+1, maxVal)
		t2 = clamp(near+1+factor/64, t1, maxVal)
		t3 = clamp(near+1+factor/16, t2, maxVal)
	} else {
		// For small MAXVAL, use special formula
		t1 = clamp(near+1+max((maxVal+1)/16, 1), near+1, maxVal)
		t2 = clamp(near+1+max((maxVal+1)/8, 1), t1, maxVal)
		t3 = clamp(near+1+max((maxVal+1)/4, 1), t2, maxVal)
	}
	return
}

// clamp returns val constrained to [minVal, maxVal]
func clamp(val, minVal, maxVal int) int {
	return max(minVal, min(val, maxVal))
}
