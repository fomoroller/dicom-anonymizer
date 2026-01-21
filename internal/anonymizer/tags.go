package anonymizer

import "github.com/suyashkumar/dicom/pkg/tag"

// PIITagsToClear are DICOM tags to clear completely (set to empty string)
var PIITagsToClear = []tag.Tag{
	// Patient identifiers
	tag.PatientName,
	tag.PatientBirthDate,
	tag.PatientAge,
	// tag.PatientSex - KEPT for clinical relevance
	tag.PatientAddress,
	tag.PatientTelephoneNumbers,
	tag.OtherPatientIDs,
	tag.OtherPatientIDsSequence,
	tag.PatientBirthTime,
	tag.PatientMotherBirthName,
	tag.MilitaryRank,
	tag.EthnicGroup,
	tag.PatientReligiousPreference,
	tag.PatientComments,

	// Times only (dates handled separately to keep year-month)
	tag.StudyTime,
	tag.SeriesTime,
	tag.AcquisitionTime,
	tag.ContentTime,
	tag.InstanceCreationTime,

	// Institution information (InstitutionName KEPT for research tracking)
	tag.InstitutionAddress,
	tag.InstitutionalDepartmentName,
	tag.StationName,

	// Study/Series descriptions - KEPT for clinical context

	// Physician information
	tag.ReferringPhysicianName,
	tag.ReferringPhysicianAddress,
	tag.ReferringPhysicianTelephoneNumbers,
	tag.PerformingPhysicianName,
	tag.OperatorsName,
	tag.PhysiciansOfRecord,
	tag.NameOfPhysiciansReadingStudy,
	tag.RequestingPhysician,
	tag.ScheduledPerformingPhysicianName,

	// Other identifiers
	tag.AccessionNumber,
	tag.RequestAttributesSequence,
	tag.PerformedProcedureStepID,
	tag.ScheduledProcedureStepID,
	tag.StudyID,
}

// DateTagsToTruncate are DICOM date tags to truncate to YYYYMM01
var DateTagsToTruncate = []tag.Tag{
	tag.StudyDate,
	tag.SeriesDate,
	tag.AcquisitionDate,
	tag.ContentDate,
	tag.InstanceCreationDate,
}

// UltrasoundPIITags are additional PII tags specific to ultrasound
var UltrasoundPIITags = []tag.Tag{
	tag.PatientName,
	tag.PatientBirthDate,
	tag.PatientAge,
	// tag.PatientSex - KEPT for clinical relevance
	tag.PatientAddress,
	tag.PatientTelephoneNumbers,
	tag.OtherPatientIDs,
	tag.StudyTime,
	tag.SeriesTime,
	tag.AcquisitionTime,
	tag.ContentTime,
	// tag.InstitutionName - KEPT for research tracking
	tag.InstitutionAddress,
	tag.InstitutionalDepartmentName,
	tag.StationName,
	tag.ReferringPhysicianName,
	tag.PerformingPhysicianName,
	tag.OperatorsName,
	tag.PhysiciansOfRecord,
	tag.NameOfPhysiciansReadingStudy,
	tag.AccessionNumber,
	tag.StudyID,
	// tag.StudyDescription - KEPT for clinical context
	// tag.SeriesDescription - KEPT for clinical context
}

// UltrasoundDateTags are date tags for ultrasound
var UltrasoundDateTags = []tag.Tag{
	tag.StudyDate,
	tag.SeriesDate,
	tag.AcquisitionDate,
	tag.ContentDate,
}
