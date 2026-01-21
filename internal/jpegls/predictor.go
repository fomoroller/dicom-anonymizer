package jpegls

// Predict computes the predicted value using the MED (Median Edge Detection)
// predictor as specified in ITU-T T.87 Section A.4.
//
// The neighbors are:
//
//	c b d
//	a x
//
// where x is the current sample being predicted.
//
// MED predictor:
//   - If c >= max(a, b), predict min(a, b) (horizontal edge)
//   - If c <= min(a, b), predict max(a, b) (vertical edge)
//   - Otherwise, predict a + b - c (no edge)
func Predict(a, b, c int) int {
	if c >= max(a, b) {
		return min(a, b)
	}
	if c <= min(a, b) {
		return max(a, b)
	}
	return a + b - c
}

// PredictWithCorrection computes the predicted value and applies
// context-based bias correction.
//
// correction is the bias correction value C[Q] from the context model.
// The corrected prediction is clamped to [0, maxVal].
func PredictWithCorrection(a, b, c, correction, maxVal int) int {
	px := Predict(a, b, c)
	px += correction
	return clampSample(px, maxVal)
}

// clampSample clamps a sample value to [0, maxVal]
func clampSample(val, maxVal int) int {
	if val < 0 {
		return 0
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// ComputeGradients computes the local gradients for context determination.
// The gradients are computed from the causal template:
//
//	c b d
//	a x
//
// Returns:
//
//	g1 = d - b (diagonal gradient)
//	g2 = b - c (vertical gradient)
//	g3 = c - a (horizontal gradient)
func ComputeGradients(a, b, c, d int) (g1, g2, g3 int) {
	g1 = d - b
	g2 = b - c
	g3 = c - a
	return
}

// IsRunMode returns true if all gradients are zero, indicating
// a uniform region suitable for run-length encoding.
func IsRunMode(g1, g2, g3 int) bool {
	return g1 == 0 && g2 == 0 && g3 == 0
}

// NeighborGetter provides access to neighboring pixel values
// during encoding. It handles boundary conditions automatically.
type NeighborGetter struct {
	pixels     []int // row-major pixel data
	width      int
	height     int
	defaultVal int
}

// NewNeighborGetter creates a new neighbor getter for the given pixel data.
// defaultVal is the value used for out-of-bounds neighbors (2^(P-1) for lossless).
func NewNeighborGetter(pixels []int, width, height, defaultVal int) *NeighborGetter {
	return &NeighborGetter{
		pixels:     pixels,
		width:      width,
		height:     height,
		defaultVal: defaultVal,
	}
}

// Get returns the pixel value at (x, y).
// Returns defaultVal for out-of-bounds coordinates (boundary handling).
func (ng *NeighborGetter) Get(x, y int) int {
	if x < 0 || y < 0 || x >= ng.width || y >= ng.height {
		return ng.defaultVal
	}
	return ng.pixels[y*ng.width+x]
}

// GetNeighbors returns the four neighbors (a, b, c, d) for position (x, y).
//
//	c b d
//	a x
//
// Boundary conditions per ITU-T T.87:
// - First row (y=0): b=c=d=defaultVal, a=previous pixel (or defaultVal if first column)
// - First column (x=0): a=b, c=b (extended from above)
// - Last column: d=b (no pixel to the right above)
func (ng *NeighborGetter) GetNeighbors(x, y int) (a, b, c, d int) {
	if y == 0 {
		// First row: all upper neighbors are defaultVal (half maxval)
		if x == 0 {
			a = ng.defaultVal
		} else {
			a = ng.Get(x-1, y)
		}
		b = ng.defaultVal
		c = ng.defaultVal
		d = ng.defaultVal
		return
	}

	// Not first row
	b = ng.Get(x, y-1)

	if x == 0 {
		// First column
		a = b // extend from above
		c = b // extend from above
	} else {
		a = ng.Get(x-1, y)
		c = ng.Get(x-1, y-1)
	}

	if x == ng.width-1 {
		// Last column
		d = b // extend from b
	} else {
		d = ng.Get(x+1, y-1)
	}

	return
}

// GetPreviousSample returns the reconstructed sample at (x-1, y).
// This is needed for run mode to get the reference value.
func (ng *NeighborGetter) GetPreviousSample(x, y int) int {
	if x == 0 {
		if y == 0 {
			return ng.defaultVal
		}
		return ng.Get(ng.width-1, y-1)
	}
	return ng.Get(x-1, y)
}

// SetPixel sets the pixel value at (x, y).
// Used to store reconstructed values during encoding.
func (ng *NeighborGetter) SetPixel(x, y, val int) {
	if x >= 0 && y >= 0 && x < ng.width && y < ng.height {
		ng.pixels[y*ng.width+x] = val
	}
}
