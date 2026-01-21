package progress

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FileStatus represents the processing status of a file
type FileStatus string

const (
	StatusSuccess FileStatus = "success"
	StatusError   FileStatus = "error"
)

// FileEntry represents a processed file entry
type FileEntry struct {
	Status    FileStatus `json:"status"`
	Hash      string     `json:"hash"`
	Output    string     `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	Timestamp string     `json:"timestamp"`
}

// TrackerData is the JSON structure for persistence
type TrackerData struct {
	Files   map[string]*FileEntry `json:"files"`
	Updated string                `json:"updated"`
	Summary struct {
		Success int `json:"success"`
		Error   int `json:"error"`
		Total   int `json:"total"`
	} `json:"summary"`
}

// Tracker tracks processing progress for resumable runs.
type Tracker struct {
	mu           sync.Mutex
	progressFile string
	processed    map[string]*FileEntry
}

// NewTracker creates a new progress tracker.
func NewTracker(progressFile string) *Tracker {
	t := &Tracker{
		progressFile: progressFile,
		processed:    make(map[string]*FileEntry),
	}

	if progressFile != "" {
		t.load()
	}

	return t
}

func (t *Tracker) load() {
	data, err := os.ReadFile(t.progressFile)
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var trackerData TrackerData
	if err := json.Unmarshal(data, &trackerData); err != nil {
		fmt.Printf("Warning: Could not load progress file: %v\n", err)
		return
	}

	t.processed = trackerData.Files
	if t.processed == nil {
		t.processed = make(map[string]*FileEntry)
	}

	successCount := t.countStatus(StatusSuccess)
	errorCount := t.countStatus(StatusError)
	fmt.Printf("Loaded progress: %d succeeded, %d failed\n", successCount, errorCount)
}

func (t *Tracker) save() {
	if t.progressFile == "" {
		return
	}

	trackerData := TrackerData{
		Files:   t.processed,
		Updated: time.Now().Format(time.RFC3339),
	}
	trackerData.Summary.Success = t.countStatus(StatusSuccess)
	trackerData.Summary.Error = t.countStatus(StatusError)
	trackerData.Summary.Total = len(t.processed)

	data, err := json.MarshalIndent(trackerData, "", "  ")
	if err != nil {
		fmt.Printf("Warning: Could not marshal progress data: %v\n", err)
		return
	}

	if err := os.WriteFile(t.progressFile, data, 0644); err != nil {
		fmt.Printf("Warning: Could not save progress: %v\n", err)
	}
}

func (t *Tracker) countStatus(status FileStatus) int {
	count := 0
	for _, entry := range t.processed {
		if entry.Status == status {
			count++
		}
	}
	return count
}

// fileHash creates a quick hash based on file size and modification time
func fileHash(filePath string) string {
	info, err := os.Stat(filePath)
	if err != nil {
		return ""
	}
	hashInput := fmt.Sprintf("%d_%d", info.Size(), info.ModTime().Unix())
	hash := md5.Sum([]byte(hashInput))
	return fmt.Sprintf("%x", hash[:4])
}

// IsProcessed checks if a file has been successfully processed.
func (t *Tracker) IsProcessed(filePath string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.processed[filePath]
	if !ok {
		return false
	}

	if entry.Status != StatusSuccess {
		return false
	}

	currentHash := fileHash(filePath)
	return entry.Hash == currentHash
}

// MarkSuccess marks a file as successfully processed.
func (t *Tracker) MarkSuccess(filePath, outputPath string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.processed[filePath] = &FileEntry{
		Status:    StatusSuccess,
		Hash:      fileHash(filePath),
		Output:    outputPath,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	t.save()
}

// MarkError marks a file as failed.
func (t *Tracker) MarkError(filePath, errorMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.processed[filePath] = &FileEntry{
		Status:    StatusError,
		Hash:      fileHash(filePath),
		Error:     errorMsg,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	t.save()
}

// ClearFailed removes all failed entries for retry.
func (t *Tracker) ClearFailed() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := 0
	for key, entry := range t.processed {
		if entry.Status == StatusError {
			delete(t.processed, key)
			count++
		}
	}

	if count > 0 {
		t.save()
		fmt.Printf("Cleared %d failed entries for retry\n", count)
	}

	return count
}

// GetStats returns success and error counts.
func (t *Tracker) GetStats() (success, errors int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.countStatus(StatusSuccess), t.countStatus(StatusError)
}
