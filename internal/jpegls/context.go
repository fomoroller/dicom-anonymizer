package jpegls

// Context holds the adaptive statistics for a single context.
// These values are updated after each sample is encoded.
type Context struct {
	// A is the accumulated sum of absolute prediction errors
	A int
	// B is the accumulated sum of prediction errors (bias)
	B int
	// C is the bias correction value
	C int
	// N is the occurrence count
	N int
}

// ContextModel manages all encoding contexts and their statistics.
type ContextModel struct {
	// Regular mode contexts (365 total)
	contexts [ContextCount]Context

	// Run mode contexts (2 total)
	runContexts [RunContextCount]Context

	// Run index for J-table (0-31)
	runIndex int

	// Parameters
	params *Params
}

// NewContextModel creates a new context model with initialized statistics.
func NewContextModel(params *Params) *ContextModel {
	cm := &ContextModel{
		params:   params,
		runIndex: 0,
	}

	// Initialize all regular contexts (ITU-T T.87 A.2.1)
	initA := max((params.Range+32)/64, 2)
	for i := range cm.contexts {
		cm.contexts[i] = Context{
			A: initA,
			B: 0,
			C: 0,
			N: 1,
		}
	}

	// Initialize run contexts
	for i := range cm.runContexts {
		cm.runContexts[i] = Context{
			A: initA,
			B: 0,
			C: 0,
			N: 1,
		}
	}

	return cm
}

// QuantizeGradient quantizes a gradient value to the range [-4, 4].
// This is the core of JPEG-LS context determination.
// Per ITU-T T.87 Table A.7:
//
//	D < -T3          => Q = -4
//	-T3 ≤ D < -T2    => Q = -3
//	-T2 ≤ D < -T1    => Q = -2
//	-T1 ≤ D < 0      => Q = -1
//	D = 0            => Q = 0
//	0 < D ≤ T1       => Q = 1
//	T1 < D ≤ T2      => Q = 2
//	T2 < D ≤ T3      => Q = 3
//	T3 < D           => Q = 4
func QuantizeGradient(g, t1, t2, t3 int) int {
	if g < -t3 {
		return -4
	}
	if g < -t2 {
		return -3
	}
	if g < -t1 {
		return -2
	}
	if g < 0 {
		return -1
	}
	if g == 0 {
		return 0
	}
	if g <= t1 {
		return 1
	}
	if g <= t2 {
		return 2
	}
	if g <= t3 {
		return 3
	}
	return 4
}

// GetContextIndex computes the context index Q from quantized gradients.
// The index is in the range [0, 364].
//
// Per ITU-T T.87, the context is mapped using:
//
//	Q = 81*q1 + 9*q2 + q3 (before sign handling)
//
// Sign handling ensures the first non-zero gradient is positive.
func GetContextIndex(q1, q2, q3 int) (idx int, sign int) {
	sign = 1

	// Ensure the first non-zero gradient is positive (sign flip)
	if q1 < 0 || (q1 == 0 && q2 < 0) || (q1 == 0 && q2 == 0 && q3 < 0) {
		q1, q2, q3 = -q1, -q2, -q3
		sign = -1
	}

	// Map quantized gradients to context index
	// q1 is in [0, 4], q2 and q3 are in [-4, 4]
	// Context index = q1 * 81 + (q2 + 4) * 9 + (q3 + 4)
	// This gives a range of [0, 364]
	idx = q1*81 + (q2+4)*9 + (q3 + 4)

	return
}

// GetContext returns the context for the given index.
func (cm *ContextModel) GetContext(idx int) *Context {
	if idx < 0 || idx >= ContextCount {
		return &cm.contexts[0]
	}
	return &cm.contexts[idx]
}

// GetRunContext returns the run mode context for the given index (0 or 1).
func (cm *ContextModel) GetRunContext(idx int) *Context {
	if idx < 0 || idx >= RunContextCount {
		return &cm.runContexts[0]
	}
	return &cm.runContexts[idx]
}

// ComputeK computes the Golomb parameter k for the given context.
// k = ceil(log2(A[Q]/N[Q]))
func (ctx *Context) ComputeK(maxK int) int {
	if ctx.N == 0 {
		return 0
	}

	// k such that 2^k >= A/N, i.e., k = ceil(log2(A/N))
	k := 0
	temp := ctx.N
	for temp < ctx.A {
		temp <<= 1
		k++
	}

	if k > maxK {
		return maxK
	}
	return k
}

// UpdateStatistics updates the context statistics after encoding a sample.
// errval is the (possibly mapped) prediction error.
func (ctx *Context) UpdateStatistics(errval, near, reset int) {
	// Update B (bias accumulator)
	ctx.B += errval

	// Update A (error magnitude accumulator)
	absErr := errval
	if absErr < 0 {
		absErr = -absErr
	}
	ctx.A += absErr

	// Check for reset (when N reaches RESET threshold)
	if ctx.N == reset {
		ctx.A = (ctx.A + 1) >> 1
		if ctx.B >= 0 {
			ctx.B = (ctx.B + 1) >> 1
		} else {
			ctx.B = -((1 - ctx.B) >> 1)
		}
		ctx.N = (ctx.N + 1) >> 1
	}

	// Increment occurrence count
	ctx.N++

	// Update bias correction C
	if ctx.B <= -ctx.N {
		ctx.B += ctx.N
		if ctx.C > MinC {
			ctx.C--
		}
		if ctx.B <= -ctx.N {
			ctx.B = -ctx.N + 1
		}
	} else if ctx.B > 0 {
		ctx.B -= ctx.N
		if ctx.C < MaxC {
			ctx.C++
		}
		if ctx.B > 0 {
			ctx.B = 0
		}
	}
}

// GetBiasCorrection returns the current bias correction value.
func (ctx *Context) GetBiasCorrection() int {
	return ctx.C
}

// GetRunIndex returns the current run index for J-table lookup.
func (cm *ContextModel) GetRunIndex() int {
	return cm.runIndex
}

// IncrementRunIndex increments the run index (after successful run).
func (cm *ContextModel) IncrementRunIndex() {
	if cm.runIndex < len(JTable)-1 {
		cm.runIndex++
	}
}

// DecrementRunIndex decrements the run index (after run interruption).
func (cm *ContextModel) DecrementRunIndex() {
	if cm.runIndex > 0 {
		cm.runIndex--
	}
}

// ResetRunIndex resets the run index to 0.
func (cm *ContextModel) ResetRunIndex() {
	cm.runIndex = 0
}

// ComputeContextFromGradients computes the context index from raw gradients.
func (cm *ContextModel) ComputeContextFromGradients(g1, g2, g3 int) (idx int, sign int) {
	q1 := QuantizeGradient(g1, cm.params.T1, cm.params.T2, cm.params.T3)
	q2 := QuantizeGradient(g2, cm.params.T1, cm.params.T2, cm.params.T3)
	q3 := QuantizeGradient(g3, cm.params.T1, cm.params.T2, cm.params.T3)
	return GetContextIndex(q1, q2, q3)
}
