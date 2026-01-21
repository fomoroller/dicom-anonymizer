package dicom

import (
	"fmt"
	"os"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/tag"
)

// Dataset wraps a DICOM dataset for easier access
type Dataset struct {
	Data     dicom.Dataset
	FilePath string
}

// ReadDicom reads a DICOM file and returns the dataset.
func ReadDicom(path string) (*Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %w", err)
	}

	ds, err := dicom.Parse(file, info.Size(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not parse DICOM: %w", err)
	}

	return &Dataset{
		Data:     ds,
		FilePath: path,
	}, nil
}

// ReadDicomMetadataOnly reads only the metadata (no pixel data).
func ReadDicomMetadataOnly(path string) (*Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %w", err)
	}

	ds, err := dicom.Parse(file, info.Size(), nil, dicom.SkipPixelData())
	if err != nil {
		return nil, fmt.Errorf("could not parse DICOM: %w", err)
	}

	return &Dataset{
		Data:     ds,
		FilePath: path,
	}, nil
}

// GetString returns a string value for a tag, or empty string if not found.
func (d *Dataset) GetString(t tag.Tag) string {
	elem, err := d.Data.FindElementByTag(t)
	if err != nil {
		return ""
	}

	if elem.Value == nil {
		return ""
	}

	strings := elem.Value.GetValue()
	if strings == nil {
		return ""
	}

	switch v := strings.(type) {
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	case string:
		return v
	}

	return fmt.Sprintf("%v", strings)
}

// GetPatientName returns the patient name.
func (d *Dataset) GetPatientName() string {
	return d.GetString(tag.PatientName)
}

// GetPatientID returns the patient ID.
func (d *Dataset) GetPatientID() string {
	return d.GetString(tag.PatientID)
}

// GetPatientBirthDate returns the patient DOB.
func (d *Dataset) GetPatientBirthDate() string {
	return d.GetString(tag.PatientBirthDate)
}

// GetTransferSyntax returns the transfer syntax UID.
func (d *Dataset) GetTransferSyntax() string {
	return d.GetString(tag.TransferSyntaxUID)
}

// GetModality returns the DICOM modality (e.g., "US", "CT", "MR", "CR", "DX").
func (d *Dataset) GetModality() string {
	return d.GetString(tag.Modality)
}

// IsUltrasound returns true if this is an ultrasound image.
func (d *Dataset) IsUltrasound() bool {
	modality := d.GetModality()
	return modality == "US" || modality == "IVUS" // Intravascular ultrasound
}
