package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MatchMethod indicates how a patient was matched
type MatchMethod string

const (
	MatchIdentity MatchMethod = "identity"
	MatchPID      MatchMethod = "pid"
	MatchNone     MatchMethod = "none"
)

// ReverseMapEntry stores reverse lookup info for audit trail
type ReverseMapEntry struct {
	IdentityHashes []string `json:"identity_hashes"`
	PatientIDs     []string `json:"patient_ids"`
}

// MapperData is the JSON structure for persistence
type MapperData struct {
	IdentityMap map[string]string           `json:"identity_map"`
	PIDMap      map[string]string           `json:"pid_map"`
	ReverseMap  map[string]*ReverseMapEntry `json:"reverse_map"`
	Counter     int                         `json:"counter"`
	Updated     string                      `json:"updated"`
	Note        string                      `json:"note"`
}

// PseudonymizationMapper manages consistent patient ID mapping across datasets.
type PseudonymizationMapper struct {
	mu          sync.Mutex
	mappingFile string
	salt        string
	identityMap map[string]string           // identity_hash -> anon_id
	pidMap      map[string]string           // patient_id -> anon_id
	reverseMap  map[string]*ReverseMapEntry // anon_id -> info
	counter     int
}

// NewPseudonymizationMapper creates a new mapper, loading from file if it exists.
func NewPseudonymizationMapper(mappingFile, salt string) *PseudonymizationMapper {
	m := &PseudonymizationMapper{
		mappingFile: mappingFile,
		salt:        salt,
		identityMap: make(map[string]string),
		pidMap:      make(map[string]string),
		reverseMap:  make(map[string]*ReverseMapEntry),
		counter:     0,
	}

	if mappingFile != "" {
		m.load()
	}

	return m
}

func (m *PseudonymizationMapper) load() {
	data, err := os.ReadFile(m.mappingFile)
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var mapData MapperData
	if err := json.Unmarshal(data, &mapData); err != nil {
		fmt.Printf("Warning: Could not load mapping file: %v\n", err)
		return
	}

	m.identityMap = mapData.IdentityMap
	if m.identityMap == nil {
		m.identityMap = make(map[string]string)
	}

	m.pidMap = mapData.PIDMap
	if m.pidMap == nil {
		m.pidMap = make(map[string]string)
	}

	m.reverseMap = mapData.ReverseMap
	if m.reverseMap == nil {
		m.reverseMap = make(map[string]*ReverseMapEntry)
	}

	m.counter = mapData.Counter

	// Count unique patients
	uniqueIDs := make(map[string]bool)
	for _, id := range m.identityMap {
		uniqueIDs[id] = true
	}
	for _, id := range m.pidMap {
		uniqueIDs[id] = true
	}

	fmt.Printf("Loaded %d patient mappings from %s\n", len(uniqueIDs), m.mappingFile)
}

func (m *PseudonymizationMapper) save() {
	if m.mappingFile == "" {
		return
	}

	// Ensure parent directory exists
	dir := filepath.Dir(m.mappingFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: Could not create mapping directory: %v\n", err)
		return
	}

	mapData := MapperData{
		IdentityMap: m.identityMap,
		PIDMap:      m.pidMap,
		ReverseMap:  m.reverseMap,
		Counter:     m.counter,
		Updated:     time.Now().Format(time.RFC3339),
		Note:        "identity_map uses hash(Name+DOB), pid_map is fallback for missing identity",
	}

	data, err := json.MarshalIndent(mapData, "", "  ")
	if err != nil {
		fmt.Printf("Warning: Could not marshal mapping data: %v\n", err)
		return
	}

	if err := os.WriteFile(m.mappingFile, data, 0644); err != nil {
		fmt.Printf("Warning: Could not save mapping file: %v\n", err)
	}
}

func (m *PseudonymizationMapper) generateID() string {
	m.counter++
	return fmt.Sprintf("ANON-%06d", m.counter)
}

func (m *PseudonymizationMapper) updateReverseMap(anonID string, identityHash, patientID string) {
	if m.reverseMap[anonID] == nil {
		m.reverseMap[anonID] = &ReverseMapEntry{
			IdentityHashes: []string{},
			PatientIDs:     []string{},
		}
	}

	entry := m.reverseMap[anonID]

	if identityHash != "" && !contains(entry.IdentityHashes, identityHash) {
		entry.IdentityHashes = append(entry.IdentityHashes, identityHash)
	}

	if patientID != "" && !contains(entry.PatientIDs, patientID) {
		entry.PatientIDs = append(entry.PatientIDs, patientID)
	}
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// GetAnonID gets or creates an anonymized ID for a patient.
// Uses Name+DOB for identity matching when available, falls back to PatientID.
func (m *PseudonymizationMapper) GetAnonID(patientID, patientName, patientDOB string) (string, MatchMethod) {
	m.mu.Lock()
	defer m.mu.Unlock()

	patientID = strings.TrimSpace(patientID)
	patientName = strings.TrimSpace(patientName)
	patientDOB = strings.TrimSpace(patientDOB)

	// Try identity-based matching first
	if IsValidIdentity(patientName, patientDOB) {
		identityHash := CreateIdentityHash(patientName, patientDOB, m.salt)

		// Check if identity already mapped
		if anonID, ok := m.identityMap[identityHash]; ok {
			// Also store PID mapping for reference
			if patientID != "" {
				if _, exists := m.pidMap[patientID]; !exists {
					m.pidMap[patientID] = anonID
					m.save()
				}
			}
			return anonID, MatchIdentity
		}

		// Check if PID was already mapped (link identity to existing)
		if anonID, ok := m.pidMap[patientID]; ok {
			m.identityMap[identityHash] = anonID
			m.updateReverseMap(anonID, identityHash, patientID)
			m.save()
			return anonID, MatchIdentity
		}

		// New patient - create new ID
		anonID := m.generateID()
		m.identityMap[identityHash] = anonID
		if patientID != "" {
			m.pidMap[patientID] = anonID
		}
		m.updateReverseMap(anonID, identityHash, patientID)
		m.save()
		return anonID, MatchIdentity
	}

	// Fallback to PatientID-based matching
	if patientID != "" {
		if anonID, ok := m.pidMap[patientID]; ok {
			return anonID, MatchPID
		}

		anonID := m.generateID()
		m.pidMap[patientID] = anonID
		m.updateReverseMap(anonID, "", patientID)
		m.save()
		return anonID, MatchPID
	}

	// No identity and no PID - generate unique ID
	anonID := m.generateID()
	m.save()
	return anonID, MatchNone
}

// Stats returns mapping statistics
type Stats struct {
	TotalPatients   int
	IdentityMatched int
	PIDFallback     int
}

// GetStats returns mapping statistics
func (m *PseudonymizationMapper) GetStats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	identityMatched := len(m.identityMap)

	// Count PIDs that don't have corresponding identity matches
	identityValues := make(map[string]bool)
	for _, v := range m.identityMap {
		identityValues[v] = true
	}

	pidOnly := 0
	for _, v := range m.pidMap {
		if !identityValues[v] {
			pidOnly++
		}
	}

	return Stats{
		TotalPatients:   len(m.reverseMap),
		IdentityMatched: identityMatched,
		PIDFallback:     pidOnly,
	}
}
