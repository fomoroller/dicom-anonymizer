package jpegls

import (
	"bytes"
	"fmt"
)

// Encoder encodes image data using JPEG-LS compression.
type Encoder struct {
	params  *Params
	cm      *ContextModel
	runEnc  *RunModeEncoder
	width   int
	height  int
	samples int // samples per pixel (1 for grayscale, 3 for RGB)
	bpp     int // bits per pixel/sample
}

// NewEncoder creates a new JPEG-LS encoder.
func NewEncoder(width, height, samples, bpp int) *Encoder {
	params := NewParams(bpp, 0) // Lossless
	cm := NewContextModel(params)
	runEnc := NewRunModeEncoder(cm, params)

	return &Encoder{
		params:  params,
		cm:      cm,
		runEnc:  runEnc,
		width:   width,
		height:  height,
		samples: samples,
		bpp:     bpp,
	}
}

// Encode compresses the given pixel data and returns the JPEG-LS bitstream.
// pixels should be in row-major order.
// For grayscale, pixels is a single []int.
// For multi-component, pixels should be interleaved (R,G,B,R,G,B,...).
func (e *Encoder) Encode(pixels []int) ([]byte, error) {
	if len(pixels) != e.width*e.height*e.samples {
		return nil, fmt.Errorf("pixel count mismatch: expected %d, got %d",
			e.width*e.height*e.samples, len(pixels))
	}

	var buf bytes.Buffer

	// Write JPEG-LS header
	frameInfo := FrameInfo{
		Width:          e.width,
		Height:         e.height,
		BitsPerSample:  e.bpp,
		ComponentCount: e.samples,
	}

	scanInfo := ScanInfo{
		Near:      e.params.Near,
		ILV:       0, // Default; may change for multi-component scans
		Pt:        0,
		MaxVal:    e.params.MaxVal,
		T1:        e.params.T1,
		T2:        e.params.T2,
		T3:        e.params.T3,
		Reset:     e.params.Reset,
		UsePreset: false, // Use default parameters
	}

	WriteSOI(&buf)
	WriteSOF55(&buf, frameInfo)
	if scanInfo.UsePreset {
		WriteLSEPreset(&buf, scanInfo)
	}

	// Encode each component separately (non-interleaved)
	if e.samples == 1 {
		// Single component (grayscale)
		WriteSOSComponents(&buf, scanInfo, []int{1})
		if err := e.encodeComponent(&buf, pixels); err != nil {
			return nil, err
		}
	} else {
		// Multi-component: sample-interleaved single scan (ILV=2)
		scanInfo.ILV = 2
		componentIDs := make([]int, e.samples)
		for i := 0; i < e.samples; i++ {
			componentIDs[i] = i + 1
		}
		WriteSOSComponents(&buf, scanInfo, componentIDs)
		if err := e.encodeSampleInterleaved(&buf, pixels); err != nil {
			return nil, err
		}
	}

	// Write trailer
	WriteJPEGLSTrailer(&buf)

	return buf.Bytes(), nil
}

// encodeComponent encodes a single image component (plane).
func (e *Encoder) encodeComponent(buf *bytes.Buffer, pixels []int) error {
	// Create bit writer
	bw := NewBitWriter(buf)

	// Create a working copy for reconstruction
	recon := make([]int, len(pixels))
	copy(recon, pixels)

	defaultVal := (e.params.MaxVal + 1) / 2
	ng := NewNeighborGetter(recon, e.width, e.height, defaultVal)

	// Process each row
	for y := 0; y < e.height; y++ {
		// RUNindex is reset at the start of each line (A.2.1 in ITU-T T.87)
		e.cm.ResetRunIndex()

		x := 0
		for x < e.width {
			// Get neighbors for current position
			a, b, c, d := ng.GetNeighbors(x, y)

			// Compute gradients
			g1, g2, g3 := ComputeGradients(a, b, c, d)

			// Check for run mode
			if DetectRunMode(g1, g2, g3) && x < e.width {
				// Run mode: encode a sequence of similar pixels
				consumed := e.runEnc.EncodeRun(bw, ng.pixels, x, y, e.width, a)
				x += consumed
			} else {
				// Regular mode: encode single pixel
				e.encodeRegularSample(bw, ng, x, y, a, b, c, g1, g2, g3)
				x++
			}
		}
	}

	// Flush remaining bits
	return bw.Flush()
}

// encodeSampleInterleaved encodes multi-component images in ILV=2 mode.
// Each pixel is encoded with components in sequence.
func (e *Encoder) encodeSampleInterleaved(buf *bytes.Buffer, pixels []int) error {
	bw := NewBitWriter(buf)

	defaultVal := (e.params.MaxVal + 1) / 2
	componentSize := e.width * e.height

	recon := make([][]int, e.samples)
	ngs := make([]*NeighborGetter, e.samples)
	cms := make([]*ContextModel, e.samples)

	for comp := 0; comp < e.samples; comp++ {
		compPixels := make([]int, componentSize)
		for i := 0; i < componentSize; i++ {
			compPixels[i] = pixels[i*e.samples+comp]
		}

		recon[comp] = compPixels
		ngs[comp] = NewNeighborGetter(recon[comp], e.width, e.height, defaultVal)
		cms[comp] = NewContextModel(e.params)
	}

	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			for comp := 0; comp < e.samples; comp++ {
				ng := ngs[comp]
				cm := cms[comp]

				// RUNindex resets once per image line.
				if x == 0 {
					cm.ResetRunIndex()
				}

				a, b, c, d := ng.GetNeighbors(x, y)
				g1, g2, g3 := ComputeGradients(a, b, c, d)

				// Run mode is not used for interleaved multi-component scans.
				e.encodeRegularSampleWithContext(bw, ng, cm, x, y, a, b, c, g1, g2, g3)
			}
		}
	}

	return bw.Flush()
}

// encodeRunMode encodes pixels using run-length mode.
func (e *Encoder) encodeRunMode(bw *BitWriter, ng *NeighborGetter, x, y, refVal int) int {
	// Get the pixels for run detection
	pixels := ng.pixels
	startIdx := y*e.width + x

	// Count run length
	runLength := 0
	remaining := e.width - x

	for i := 0; i < remaining; i++ {
		if pixels[startIdx+i] == refVal {
			runLength++
		} else {
			break
		}
	}

	// Encode run segments using J-table
	encoded := 0
	for encoded < runLength {
		runIdx := e.cm.GetRunIndex()
		if runIdx >= len(JTable) {
			runIdx = len(JTable) - 1
		}

		rk := JTable[runIdx].RK
		segmentSize := 1 << rk

		if runLength-encoded >= segmentSize {
			// Complete segment
			bw.WriteBit(1)
			encoded += segmentSize
			e.cm.IncrementRunIndex()
		} else {
			// Partial segment
			remainder := runLength - encoded
			bw.WriteBit(0)
			bw.WriteBits(remainder, rk)
			encoded = runLength
			e.cm.ResetRunIndex()
		}
	}

	// Check if run ended before end of row
	if runLength < remaining {
		// Encode the run interruption sample
		interruptIdx := startIdx + runLength
		sample := pixels[interruptIdx]

		// Get Rb (pixel above the interruption)
		rb := refVal
		if y > 0 {
			rb = ng.Get(x+runLength, y-1)
		}

		e.encodeRunInterruption(bw, sample, refVal, rb)

		return runLength + 1
	}

	return runLength
}

// encodeRunInterruption encodes the sample that interrupts a run.
func (e *Encoder) encodeRunInterruption(bw *BitWriter, sample, ra, rb int) {
	// Determine context (0 or 1 based on |Ra-Rb|)
	ctxIdx := 0
	diff := ra - rb
	if diff < 0 {
		diff = -diff
	}
	if diff > e.params.Near {
		ctxIdx = 1
	}

	// Get prediction and sign
	predicted := rb
	sign := 1
	if ra < rb {
		predicted = ra
		sign = -1
	} else if ra > rb {
		// Keep predicted = rb, sign = 1
	}

	// Compute error
	errval := sample - predicted
	errval *= sign

	// Modulo reduction
	errval = ReduceError(errval, e.params.Range)

	// Map error
	mapped := MapErrorValue(errval, e.params.Near)

	// Get run context
	ctx := e.cm.GetRunContext(ctxIdx)

	// Compute k
	k := ctx.ComputeK(LimitK)

	// Adjust k for run mode (A.7.2 in ITU-T T.87)
	runIdx := e.cm.GetRunIndex()
	if runIdx > 0 {
		k = max(0, k-1)
	}

	// Encode using modified limit for run interruption (A.7.2.1)
	// The limit is LIMIT - J[RUNindex] - 1, where J[] is the RK value
	rk := 0
	if runIdx < len(JTable) {
		rk = JTable[runIdx].RK
	}
	limit := max(2, e.params.Limit-rk-1)
	EncodeGolomb(bw, mapped, k, limit, e.params.Qbpp)

	// Update statistics
	ctx.UpdateStatistics(errval, e.params.Near, e.params.Reset)

	// Decrement run index
	e.cm.DecrementRunIndex()
}

// encodeRegularSample encodes a single sample in regular mode.
func (e *Encoder) encodeRegularSample(bw *BitWriter, ng *NeighborGetter, x, y, a, b, c, g1, g2, g3 int) {
	// Get actual pixel value
	actual := ng.Get(x, y)

	// Get context index and sign
	idx, sign := e.cm.ComputeContextFromGradients(g1, g2, g3)
	ctx := e.cm.GetContext(idx)

	// Compute prediction with bias correction
	px := Predict(a, b, c)
	px = CorrectPrediction(px, ctx.GetBiasCorrection(), sign, e.params.MaxVal)

	// Compute error
	errval := actual - px
	errval *= sign

	// Modulo reduction
	errval = ReduceError(errval, e.params.Range)

	// Map error
	mapped := MapErrorValue(errval, e.params.Near)

	// Compute k parameter
	k := ctx.ComputeK(LimitK)

	// Encode using Golomb-Rice
	EncodeGolomb(bw, mapped, k, e.params.Limit, e.params.Qbpp)

	// Update context statistics
	ctx.UpdateStatistics(errval, e.params.Near, e.params.Reset)

	// Reconstruct sample (for use as neighbor)
	reconstructed := ReconstructSample(px, errval, sign, e.params.Near, e.params.MaxVal)
	ng.SetPixel(x, y, reconstructed)
}

func (e *Encoder) encodeRegularSampleWithContext(bw *BitWriter, ng *NeighborGetter, cm *ContextModel, x, y, a, b, c, g1, g2, g3 int) {
	// Get actual pixel value
	actual := ng.Get(x, y)

	// Get context index and sign
	idx, sign := cm.ComputeContextFromGradients(g1, g2, g3)
	ctx := cm.GetContext(idx)

	// Compute prediction with bias correction
	px := Predict(a, b, c)
	px = CorrectPrediction(px, ctx.GetBiasCorrection(), sign, e.params.MaxVal)

	// Compute error
	errval := actual - px
	errval *= sign

	// Modulo reduction
	errval = ReduceError(errval, e.params.Range)

	// Map error
	mapped := MapErrorValue(errval, e.params.Near)

	// Compute k parameter
	k := ctx.ComputeK(LimitK)

	// Encode using Golomb-Rice
	EncodeGolomb(bw, mapped, k, e.params.Limit, e.params.Qbpp)

	// Update context statistics
	ctx.UpdateStatistics(errval, e.params.Near, e.params.Reset)

	// Reconstruct sample (for use as neighbor)
	reconstructed := ReconstructSample(px, errval, sign, e.params.Near, e.params.MaxVal)
	ng.SetPixel(x, y, reconstructed)
}

// EncodeGrayscale is a convenience function to encode 8-bit grayscale data.
func EncodeGrayscale(pixels []byte, width, height int) ([]byte, error) {
	// Convert to int
	intPixels := make([]int, len(pixels))
	for i, p := range pixels {
		intPixels[i] = int(p)
	}

	enc := NewEncoder(width, height, 1, 8)
	return enc.Encode(intPixels)
}

// EncodeGrayscale16 encodes 16-bit grayscale data.
func EncodeGrayscale16(pixels []uint16, width, height, bpp int) ([]byte, error) {
	// Convert to int
	intPixels := make([]int, len(pixels))
	for i, p := range pixels {
		intPixels[i] = int(p)
	}

	enc := NewEncoder(width, height, 1, bpp)
	return enc.Encode(intPixels)
}

// EncodeFromBytes encodes pixel data from a byte slice.
// bytesPerSample should be 1 for 8-bit, 2 for 16-bit data.
// For 16-bit, little-endian byte order is assumed.
func EncodeFromBytes(data []byte, width, height, samples, bpp int) ([]byte, error) {
	bytesPerSample := (bpp + 7) / 8
	expectedLen := width * height * samples * bytesPerSample

	if len(data) != expectedLen {
		return nil, fmt.Errorf("data length mismatch: expected %d, got %d", expectedLen, len(data))
	}

	// Convert to int
	pixelCount := width * height * samples
	intPixels := make([]int, pixelCount)

	if bytesPerSample == 1 {
		for i := 0; i < pixelCount; i++ {
			intPixels[i] = int(data[i])
		}
	} else {
		// 16-bit, little-endian
		for i := 0; i < pixelCount; i++ {
			lo := data[i*2]
			hi := data[i*2+1]
			intPixels[i] = int(lo) | (int(hi) << 8)
		}
	}

	enc := NewEncoder(width, height, samples, bpp)
	return enc.Encode(intPixels)
}
