package dicom

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/tag"
)

// SetString sets a string value for a tag in the dataset.
func (d *Dataset) SetString(t tag.Tag, value string) error {
	// Find the element
	elem, err := d.Data.FindElementByTag(t)
	if err != nil {
		// Element doesn't exist, that's okay
		return nil
	}

	// Get the VR to determine how to set the value
	vr := elem.RawValueRepresentation

	// Create new value
	newValue, err := dicom.NewValue([]string{value})
	if err != nil {
		return fmt.Errorf("could not create value: %w", err)
	}

	// Create a new element with the updated value
	newElem := &dicom.Element{
		Tag:                    t,
		ValueRepresentation:    elem.ValueRepresentation,
		RawValueRepresentation: vr,
		ValueLength:            uint32(len(value)),
		Value:                  newValue,
	}

	// Replace element in dataset
	for i, e := range d.Data.Elements {
		if e.Tag == t {
			d.Data.Elements[i] = newElem
			return nil
		}
	}

	return nil
}

// ClearTag clears a tag value (sets to empty string).
func (d *Dataset) ClearTag(t tag.Tag) {
	d.SetString(t, "")
}

// TruncateDate truncates a date to YYYYMM01 format.
func (d *Dataset) TruncateDate(t tag.Tag) {
	value := d.GetString(t)
	if len(value) >= 6 {
		d.SetString(t, value[:6]+"01")
	} else if value != "" {
		d.SetString(t, "")
	}
}

// Save writes the DICOM dataset to a file.
func (d *Dataset) Save(outputPath string) error {
	return d.SaveWithOptions(outputPath, SaveOptions{})
}

// SaveOptions configures DICOM writing behavior.
type SaveOptions struct {
	// CompressJPEGLS enables JPEG-LS lossless compression for pixel data.
	// If true, the pixel data will be compressed and the transfer syntax
	// will be updated to JPEG-LS Lossless.
	CompressJPEGLS bool
}

// SaveWithOptions writes the DICOM dataset to a file with configurable options.
func (d *Dataset) SaveWithOptions(outputPath string, opts SaveOptions) error {
	// Ensure parent directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	// If JPEG-LS compression is requested, use custom writer
	if opts.CompressJPEGLS {
		return d.saveWithDcmtk(outputPath)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer file.Close()

	// Write DICOM with relaxed verification (many real-world DICOM files
	// don't strictly follow VR specifications)
	if err := dicom.Write(file, d.Data,
		dicom.SkipVRVerification(),
		dicom.SkipValueTypeVerification(),
		dicom.DefaultMissingTransferSyntax(),
	); err != nil {
		return fmt.Errorf("could not write DICOM: %w", err)
	}

	return nil
}

func (d *Dataset) saveWithDcmtk(outputPath string) error {
	_, err := exec.LookPath("dcmcjpls")
	if err != nil {
		return fmt.Errorf("dcmtk not installed (missing dcmcjpls)")
	}

	tmpFile, err := os.CreateTemp("", "dicom-uncompressed-*.dcm")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("could not create temp DICOM: %w", err)
	}
	if err := dicom.Write(file, d.Data,
		dicom.SkipVRVerification(),
		dicom.SkipValueTypeVerification(),
		dicom.DefaultMissingTransferSyntax(),
	); err != nil {
		file.Close()
		return fmt.Errorf("could not write temp DICOM: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("could not close temp DICOM: %w", err)
	}

	cmd := exec.Command("dcmcjpls", tmpPath, outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dcmcjpls failed: %s", string(output))
	}

	return nil
}

// getCompressedPixelData extracts and compresses the pixel data using JPEG-LS.
func (d *Dataset) getCompressedPixelData() ([]byte, error) {
	// Get image dimensions and format
	width, height, err := d.getImageDimensions()
	if err != nil {
		return nil, err
	}

	samples := d.getSamplesPerPixel()
	bitsAllocated := d.getBitsAllocated()

	// Extract raw pixel data
	pixelData, err := d.extractRawPixelData()
	if err != nil {
		return nil, err
	}

	// Compress using JPEG-LS
	return CompressJPEGLS(pixelData, width, height, samples, bitsAllocated)
}

// getImageDimensions returns the width and height of the image.
func (d *Dataset) getImageDimensions() (width, height int, err error) {
	rowsElem, err := d.Data.FindElementByTag(tag.Rows)
	if err != nil {
		return 0, 0, fmt.Errorf("no Rows tag found: %w", err)
	}
	colsElem, err := d.Data.FindElementByTag(tag.Columns)
	if err != nil {
		return 0, 0, fmt.Errorf("no Columns tag found: %w", err)
	}

	height = getIntValueFromElem(rowsElem)
	width = getIntValueFromElem(colsElem)

	if width == 0 || height == 0 {
		return 0, 0, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	return width, height, nil
}

// getSamplesPerPixel returns the number of samples per pixel (1 for grayscale, 3 for RGB).
func (d *Dataset) getSamplesPerPixel() int {
	elem, err := d.Data.FindElementByTag(tag.SamplesPerPixel)
	if err != nil {
		return 1 // Default to grayscale
	}
	val := getIntValueFromElem(elem)
	if val == 0 {
		return 1
	}
	return val
}

// getBitsAllocated returns the bits allocated per sample.
func (d *Dataset) getBitsAllocated() int {
	elem, err := d.Data.FindElementByTag(tag.BitsAllocated)
	if err != nil {
		return 8 // Default to 8-bit
	}
	val := getIntValueFromElem(elem)
	if val == 0 {
		return 8
	}
	return val
}

// extractRawPixelData extracts raw pixel data from the dataset.
func (d *Dataset) extractRawPixelData() ([]byte, error) {
	pixelElem, err := d.Data.FindElementByTag(tag.PixelData)
	if err != nil {
		return nil, fmt.Errorf("no pixel data found: %w", err)
	}

	pixelInfo := pixelElem.Value.GetValue()

	switch v := pixelInfo.(type) {
	case dicom.PixelDataInfo:
		// Handle native frames
		if len(v.Frames) > 0 {
			return d.extractFromNativeFrames(v)
		}
		return nil, fmt.Errorf("no frames in pixel data")

	case []byte:
		// Already raw bytes
		return v, nil

	default:
		return nil, fmt.Errorf("unsupported pixel data type: %T", pixelInfo)
	}
}

// extractFromNativeFrames converts native frame data to raw bytes.
func (d *Dataset) extractFromNativeFrames(pdi dicom.PixelDataInfo) ([]byte, error) {
	if len(pdi.Frames) == 0 {
		return nil, fmt.Errorf("no frames available")
	}

	// Get image parameters
	width, height, _ := d.getImageDimensions()
	samples := d.getSamplesPerPixel()
	bitsAllocated := d.getBitsAllocated()
	bytesPerSample := (bitsAllocated + 7) / 8

	// Calculate expected size
	pixelCount := width * height * samples
	expectedSize := pixelCount * bytesPerSample

	// For single frame, convert native data to bytes
	frame := pdi.Frames[0]
	if frame.NativeData.Data == nil {
		return nil, fmt.Errorf("native frame data is nil")
	}

	result := make([]byte, expectedSize)

	// Convert int values to bytes
	idx := 0
	for _, pixel := range frame.NativeData.Data {
		for _, sample := range pixel {
			if bytesPerSample == 1 {
				result[idx] = byte(sample)
				idx++
			} else {
				// Little-endian 16-bit
				result[idx] = byte(sample)
				result[idx+1] = byte(sample >> 8)
				idx += 2
			}
		}
	}

	return result, nil
}

// getIntValueFromElem extracts an integer value from a DICOM element.
func getIntValueFromElem(elem *dicom.Element) int {
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
