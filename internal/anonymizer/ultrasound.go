package anonymizer

import (
	"fmt"
	"os"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/frame"
	"github.com/suyashkumar/dicom/pkg/tag"

	dcm "dicom-anonymizer/internal/dicom"
)

// AnonymizeUltrasound anonymizes an ultrasound DICOM file with pixel redaction.
func AnonymizeUltrasound(inputPath, outputPath string, redactRows int, patientID string) error {
	var ds *dcm.Dataset
	var tempFile string
	var err error

	// Track if original was JPEG-LS compressed for re-compression
	wasJPEGLSCompressed := dcm.IsJPEGLSCompressed(inputPath)

	// Handle JPEG-LS compression
	if wasJPEGLSCompressed {
		tempFile, err = dcm.DecompressJPEGLS(inputPath)
		if err != nil {
			return fmt.Errorf("JPEG-LS decompression failed: %w", err)
		}
		defer os.Remove(tempFile)

		ds, err = dcm.ReadDicom(tempFile)
	} else {
		ds, err = dcm.ReadDicom(inputPath)
	}

	if err != nil {
		return fmt.Errorf("could not read DICOM: %w", err)
	}

	// Redact pixel data (top rows contain burned-in text)
	if err := redactPixels(ds, redactRows); err != nil {
		return fmt.Errorf("pixel redaction failed: %w", err)
	}

	// Set anonymized patient ID
	ds.SetString(tag.PatientID, patientID)

	// Clear PII fields
	for _, t := range UltrasoundPIITags {
		ds.ClearTag(t)
	}

	// Truncate dates to year-month only (YYYYMM01)
	for _, t := range UltrasoundDateTags {
		ds.TruncateDate(t)
	}

	// Save anonymized file with re-compression if original was compressed
	return ds.SaveWithOptions(outputPath, dcm.SaveOptions{
		CompressJPEGLS: wasJPEGLSCompressed,
	})
}

// redactPixels blacks out the top rows of pixel data
func redactPixels(ds *dcm.Dataset, redactRows int) error {
	// Find pixel data element
	pixelElem, err := ds.Data.FindElementByTag(tag.PixelData)
	if err != nil {
		return fmt.Errorf("no pixel data found: %w", err)
	}

	// Get pixel data info
	rowsElem, err := ds.Data.FindElementByTag(tag.Rows)
	if err != nil {
		return fmt.Errorf("no Rows tag found: %w", err)
	}
	colsElem, err := ds.Data.FindElementByTag(tag.Columns)
	if err != nil {
		return fmt.Errorf("no Columns tag found: %w", err)
	}
	samplesElem, _ := ds.Data.FindElementByTag(tag.SamplesPerPixel)
	bitsAllocElem, _ := ds.Data.FindElementByTag(tag.BitsAllocated)

	rows := getIntValue(rowsElem)
	cols := getIntValue(colsElem)
	samples := getIntValue(samplesElem)
	if samples == 0 {
		samples = 1
	}
	bitsAlloc := getIntValue(bitsAllocElem)
	if bitsAlloc == 0 {
		bitsAlloc = 8
	}
	bytesPerSample := bitsAlloc / 8

	// Get the pixel data
	pixelInfo := pixelElem.Value.GetValue()

	switch v := pixelInfo.(type) {
	case dicom.PixelDataInfo:
		// Handle native frames - modify in place
		if len(v.Frames) > 0 {
			for _, fr := range v.Frames {
				redactFrame(fr, rows, cols, samples, bytesPerSample, redactRows)
			}
			// Frames are modified in-place, no need to reassign
		}
	case []byte:
		// Handle raw byte data
		bytesPerRow := cols * samples * bytesPerSample
		redactBytes := min(redactRows*bytesPerRow, len(v))
		for i := 0; i < redactBytes; i++ {
			v[i] = 0
		}
	}

	return nil
}

// redactFrame blacks out the top rows of a frame
func redactFrame(f *frame.Frame, _, cols, _, _ int, redactRows int) {
	if f.NativeData.Data == nil {
		return
	}

	// For NativeData, each pixel value is stored as an int
	// Data is [][]int where outer is pixels, inner is samples
	pixelsToRedact := min(redactRows*cols, len(f.NativeData.Data))

	for i := 0; i < pixelsToRedact; i++ {
		for j := range f.NativeData.Data[i] {
			f.NativeData.Data[i][j] = 0
		}
	}
}

// getIntValue extracts an integer value from a DICOM element
func getIntValue(elem *dicom.Element) int {
	if elem == nil || elem.Value == nil {
		return 0
	}

	val := elem.Value.GetValue()
	switch v := val.(type) {
	case []int:
		if len(v) > 0 {
			return v[0]
		}
	case int:
		return v
	case []uint16:
		if len(v) > 0 {
			return int(v[0])
		}
	case uint16:
		return int(v)
	}

	return 0
}
