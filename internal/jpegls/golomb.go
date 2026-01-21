package jpegls

// MapErrorValue maps a prediction error to a non-negative value
// suitable for Golomb-Rice coding.
//
// For lossless (NEAR=0):
//
//	If error >= 0: mapped = 2 * error
//	If error < 0:  mapped = 2 * |error| - 1
//
// This interleaves positive and negative values:
//
//	error:  0, 1, -1, 2, -2, 3, -3, ...
//	mapped: 0, 2,  1, 4,  3, 6,  5, ...
func MapErrorValue(errval, near int) int {
	if near == 0 {
		// Lossless mapping
		if errval >= 0 {
			return 2 * errval
		}
		return 2*(-errval) - 1
	}

	// Near-lossless mapping (quantized errors)
	if errval >= 0 {
		return 2 * errval
	}
	return 2*(-errval) - 1
}

// UnmapErrorValue reverses the error mapping.
func UnmapErrorValue(mapped int) int {
	if mapped%2 == 0 {
		return mapped / 2
	}
	return -(mapped + 1) / 2
}

// ComputePredictionError computes and corrects the prediction error.
// The error is wrapped to the range (-RANGE/2, RANGE/2] for lossless,
// or (-RANGE/2, RANGE/2] quantized for near-lossless.
func ComputePredictionError(actual, predicted, sign, near, rangeVal int) int {
	errval := actual - predicted

	// Apply sign from context
	errval *= sign

	// For near-lossless, quantize the error
	if near > 0 {
		if errval > 0 {
			errval = (errval + near) / (2*near + 1)
		} else {
			errval = -(near - errval) / (2*near + 1)
		}
	}

	// Modulo reduction to valid range (A.4.5 in ITU-T T.87)
	if errval < 0 {
		errval += rangeVal
	}
	if errval >= (rangeVal+1)/2 {
		errval -= rangeVal
	}

	return errval
}

// ReconstructSample reconstructs the sample value from prediction and error.
// This is needed to maintain the same reference values as the decoder.
func ReconstructSample(predicted, errval, sign, near, maxVal int) int {
	// Reverse sign correction
	errval *= sign

	// For near-lossless, dequantize
	if near > 0 {
		errval *= (2*near + 1)
	}

	reconstructed := predicted + errval

	// Clamp to valid range
	if reconstructed < 0 {
		reconstructed = 0
	} else if reconstructed > maxVal {
		reconstructed = maxVal
	}

	return reconstructed
}

// EncodeGolomb encodes a mapped error value using limited-length Golomb-Rice coding.
// This is the primary entropy coding method in JPEG-LS.
//
// The encoding consists of:
//  1. Unary code for quotient q = mapped >> k (with limit)
//  2. Binary code for remainder r = mapped & ((1 << k) - 1)
func EncodeGolomb(bw *BitWriter, mapped, k, limit, qbpp int) {
	// Compute quotient and remainder
	q := mapped >> k

	// Check if we need to use limited-length coding
	if q < limit-qbpp-1 {
		// Normal Golomb-Rice: q zeros followed by 1, then k-bit remainder
		bw.WriteUnary(q)
		if k > 0 {
			bw.WriteBits(mapped&((1<<k)-1), k)
		}
	} else {
		// Limited-length Golomb: (limit-qbpp-1) zeros, then 1, then qbpp bits
		bw.WriteUnary(limit - qbpp - 1)
		bw.WriteBits(mapped-1, qbpp)
	}
}

// EncodeRegularMode encodes a sample using regular (non-run) mode.
func EncodeRegularMode(bw *BitWriter, ctx *Context, actual, predicted, sign int, params *Params) int {
	// Compute prediction error with modulo reduction
	errval := ComputePredictionError(actual, predicted, sign, params.Near, params.Range)

	// Map error to non-negative value
	mapped := MapErrorValue(errval, params.Near)

	// Compute k parameter for Golomb coding
	k := ctx.ComputeK(LimitK)

	// Encode using Golomb-Rice
	EncodeGolomb(bw, mapped, k, params.Limit, params.Qbpp)

	// Update context statistics
	ctx.UpdateStatistics(errval, params.Near, params.Reset)

	// Return reconstructed value for reference
	return ReconstructSample(predicted, errval, sign, params.Near, params.MaxVal)
}

// ReduceError applies modulo reduction to keep error in valid range.
func ReduceError(errval, rangeVal int) int {
	if errval < 0 {
		errval += rangeVal
	}
	if errval >= (rangeVal+1)/2 {
		errval -= rangeVal
	}
	return errval
}

// CorrectPrediction applies context-based bias correction to prediction.
func CorrectPrediction(px, correction, sign, maxVal int) int {
	if sign > 0 {
		px += correction
	} else {
		px -= correction
	}

	// Clamp to valid range
	if px < 0 {
		return 0
	}
	if px > maxVal {
		return maxVal
	}
	return px
}
