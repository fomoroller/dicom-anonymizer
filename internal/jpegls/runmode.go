package jpegls

// RunModeEncoder handles run-length encoding for uniform regions.
// When all gradients are zero, JPEG-LS switches to run mode which
// efficiently encodes sequences of identical (or near-identical) pixels.
type RunModeEncoder struct {
	cm     *ContextModel
	params *Params
}

// NewRunModeEncoder creates a new run mode encoder.
func NewRunModeEncoder(cm *ContextModel, params *Params) *RunModeEncoder {
	return &RunModeEncoder{
		cm:     cm,
		params: params,
	}
}

// EncodeRun encodes a run of samples starting at the given position.
// Returns the number of samples consumed.
//
// Parameters:
//   - bw: bit writer for output
//   - pixels: row-major pixel data
//   - x, y: current position
//   - width: image width
//   - refVal: the reference value (Ra) for run comparison
func (rme *RunModeEncoder) EncodeRun(bw *BitWriter, pixels []int, x, y, width, refVal int) int {
	// Count run length (how many pixels match refVal)
	runLength := 0
	remaining := width - x
	startIdx := y*width + x

	for i := 0; i < remaining; i++ {
		if rme.pixelsMatch(pixels[startIdx+i], refVal) {
			runLength++
		} else {
			break
		}
	}

	// Encode the run
	if runLength > 0 {
		rme.encodeRunSegments(bw, runLength, remaining)
	}

	// Handle run interruption (if run didn't reach end of line)
	if runLength < remaining {
		// Get the interrupting sample
		interruptingSample := pixels[startIdx+runLength]

		// Get context for run interruption
		runIdx := rme.cm.GetRunIndex()
		rb := rme.getRunInterruptionContext(x+runLength, y, width, pixels, refVal)

		// Encode the run interruption sample
		rme.encodeRunInterruptionSample(bw, interruptingSample, refVal, rb, runIdx)

		return runLength + 1
	}

	return runLength
}

// pixelsMatch checks if a pixel matches the reference value.
// For lossless, this is exact equality.
// For near-lossless, it checks if within NEAR tolerance.
func (rme *RunModeEncoder) pixelsMatch(pixel, refVal int) bool {
	if rme.params.Near == 0 {
		return pixel == refVal
	}
	diff := pixel - refVal
	if diff < 0 {
		diff = -diff
	}
	return diff <= rme.params.Near
}

// encodeRunSegments encodes run length using the J-table.
// Run lengths are encoded in segments based on the current run index.
func (rme *RunModeEncoder) encodeRunSegments(bw *BitWriter, runLength, remaining int) {
	lineRemaining := remaining

	// Encode complete segments
	for runLength > 0 {
		runIdx := rme.cm.GetRunIndex()
		if runIdx >= len(JTable) {
			runIdx = len(JTable) - 1
		}

		rk := JTable[runIdx].RK
		segmentSize := 1 << rk

		if runLength >= segmentSize {
			// Complete segment: output 1 bit
			bw.WriteBit(1)
			runLength -= segmentSize
			lineRemaining -= segmentSize

			// Increment run index for next segment
			rme.cm.IncrementRunIndex()

			if runLength == 0 {
				// End of line: no terminating segment.
				if lineRemaining == 0 {
					return
				}

				// Run ended on a segment boundary before end of line.
				runIdx = rme.cm.GetRunIndex()
				if runIdx >= len(JTable) {
					runIdx = len(JTable) - 1
				}
				rk = JTable[runIdx].RK
				bw.WriteBit(0)
				if rk > 0 {
					bw.WriteBits(0, rk)
				}
				rme.cm.ResetRunIndex()
				return
			}
		} else {
			// End of line with incomplete segment: no terminating segment.
			if runLength == lineRemaining {
				return
			}

			// Incomplete segment: output 0 bit followed by rk bits of count
			bw.WriteBit(0)
			if rk > 0 {
				bw.WriteBits(runLength, rk)
			}
			runLength = 0

			// Reset run index after incomplete segment
			rme.cm.ResetRunIndex()
		}
	}
}

// getRunInterruptionContext computes Rb for run interruption.
// Rb is the sample above the run interruption position.
func (rme *RunModeEncoder) getRunInterruptionContext(x, y, width int, pixels []int, refVal int) int {
	if y == 0 {
		return refVal
	}
	return pixels[(y-1)*width+x]
}

// encodeRunInterruptionSample encodes the sample that interrupted the run.
func (rme *RunModeEncoder) encodeRunInterruptionSample(bw *BitWriter, sample, ra, rb, runIdx int) {
	// Determine which run context to use (A.7.2 in ITU-T T.87)
	// Context index: 0 if |Ra - Rb| <= NEAR, else 1
	ctxIdx := 0
	diff := ra - rb
	if diff < 0 {
		diff = -diff
	}
	if diff > rme.params.Near {
		ctxIdx = 1
	}

	// Compute prediction (use Rb as predictor)
	predicted := rb

	// Sign determination
	sign := 1
	if ra > rb || (ra == rb && rme.params.Near > 0) {
		// No sign flip needed
	} else if ra < rb {
		sign = -1
		predicted = ra
	}

	// Get the run context
	ctx := rme.cm.GetRunContext(ctxIdx)

	// Compute prediction error
	errval := sample - predicted
	errval *= sign

	// Modulo reduction
	errval = ReduceError(errval, rme.params.Range)

	// Map error for encoding
	mapped := mapRunInterruptionError(errval, ra, rb)

	// Compute k
	k := ctx.ComputeK(LimitK)

	// For run interruption samples, we use a modified encoding
	// The map can start at 1 or 2 depending on context
	tempA := ctx.A + (ctx.N >> 1)

	// Adjust k for run mode (A.7.2 in ITU-T T.87)
	if runIdx > 0 {
		k = max(0, k-1)
	}

	// Encode using modified limit for run interruption (A.7.2.1)
	// The limit is max(2, LIMIT - J[RUNindex] - 1)
	rk := 0
	if runIdx < len(JTable) {
		rk = JTable[runIdx].RK
	}
	limit := max(2, rme.params.Limit-rk-1)
	EncodeGolomb(bw, mapped, k, limit, rme.params.Qbpp)

	// Update statistics
	if errval < 0 {
		ctx.B++
	}
	ctx.A += iabs(errval) - (tempA-ctx.A)/ctx.N

	if ctx.N == rme.params.Reset {
		ctx.A >>= 1
		ctx.N >>= 1
	}
	ctx.N++

	// Decrement run index
	rme.cm.DecrementRunIndex()
}

// mapRunInterruptionError maps the error for a run interruption sample.
// Per ITU-T T.87 A.7.2.1, the mapping depends on SIGN:
//
// If SIGN = 1 (Ra >= Rb):
//
//	errval >= 0: MErrval = 2 * errval
//	errval < 0:  MErrval = 2 * |errval| - 1
//
// If SIGN = -1 (Ra < Rb):
//
//	errval > 0:  MErrval = 2 * |errval| - 1
//	errval <= 0: MErrval = 2 * |errval|
func mapRunInterruptionError(errval, ra, rb int) int {
	if ra >= rb {
		// SIGN = 1: standard mapping
		if errval >= 0 {
			return 2 * errval
		}
		return 2*(-errval) - 1
	}
	// SIGN = -1: inverted mapping
	if errval > 0 {
		return 2*errval - 1
	}
	return 2 * (-errval)
}

// iabs returns the absolute value of an integer.
func iabs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// DetectRunMode checks if the current position should use run mode.
// Run mode is entered when all local gradients are zero.
func DetectRunMode(g1, g2, g3 int) bool {
	return g1 == 0 && g2 == 0 && g3 == 0
}
