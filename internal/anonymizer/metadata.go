package anonymizer

import (
	"github.com/suyashkumar/dicom/pkg/tag"

	dcm "dicom-anonymizer/internal/dicom"
)

// AnonymizeMetadata anonymizes metadata in a DICOM file without modifying pixels.
func AnonymizeMetadata(inputPath, outputPath, patientID string) error {
	// Read the DICOM file
	ds, err := dcm.ReadDicom(inputPath)
	if err != nil {
		return err
	}

	// Set anonymized patient ID
	ds.SetString(tag.PatientID, patientID)

	// Clear all PII tags
	for _, t := range PIITagsToClear {
		ds.ClearTag(t)
	}

	// Truncate dates to year-month only (YYYYMM01)
	for _, t := range DateTagsToTruncate {
		ds.TruncateDate(t)
	}

	// Save anonymized file
	return ds.Save(outputPath)
}
