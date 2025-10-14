package dependency

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathManager provides utilities for constructing and validating file paths
// within the shared volume structure.
//
// It enforces the flat directory structure: /data/meetings/{meeting_id}/
// All files for a meeting reside in the same directory with standardized naming:
//   - Audio chunks: chunk_0000.wav, chunk_0001.wav, ...
//   - Transcripts: chunk_0000_merged.txt, chunk_0001_merged.txt, ...
//   - Diarization: chunk_0000_segments.json, chunk_0000_speakers.json, ...
//   - Embeddings: chunk_0000_embeddings.json, ...
//   - Final outputs: merged_all.txt, polish_all.md, meeting_summary.md, ...
type PathManager struct {
	baseDir string // Base directory, e.g., "/data"
}

// NewPathManager creates a new PathManager instance.
func NewPathManager(baseDir string) *PathManager {
	return &PathManager{baseDir: baseDir}
}

// GetMeetingDir returns the root directory for a meeting.
// Example: GetMeetingDir("0925_NIEP例会") -> "/data/meetings/0925_NIEP例会"
func (pm *PathManager) GetMeetingDir(meetingID string) string {
	return filepath.Join(pm.baseDir, "meetings", meetingID)
}

// GetAudioPath returns the path for an audio file (chunk or merged).
// Examples:
//
//	GetAudioPath("meeting123", "chunk_0000.wav") -> "/data/meetings/meeting123/chunk_0000.wav"
//	GetAudioPath("meeting123", "chunk_0015.webm") -> "/data/meetings/meeting123/chunk_0015.webm"
func (pm *PathManager) GetAudioPath(meetingID string, filename string) string {
	return filepath.Join(pm.GetMeetingDir(meetingID), filename)
}

// GetTranscriptPath returns the path for a transcript file.
// Example: GetTranscriptPath("meeting123", "chunk_0000_merged.txt")
//
//	-> "/data/meetings/meeting123/chunk_0000_merged.txt"
func (pm *PathManager) GetTranscriptPath(meetingID string, filename string) string {
	return filepath.Join(pm.GetMeetingDir(meetingID), filename)
}

// GetDiarizationPath returns the path for a diarization output file.
// Examples:
//
//	GetDiarizationPath("meeting123", "chunk_0000_segments.json")
//	GetDiarizationPath("meeting123", "chunk_0000_speakers_mapped.json")
func (pm *PathManager) GetDiarizationPath(meetingID string, filename string) string {
	return filepath.Join(pm.GetMeetingDir(meetingID), filename)
}

// GetEmbeddingsPath returns the path for speaker embeddings file.
// Example: GetEmbeddingsPath("meeting123", "chunk_0000_embeddings.json")
func (pm *PathManager) GetEmbeddingsPath(meetingID string, filename string) string {
	return filepath.Join(pm.GetMeetingDir(meetingID), filename)
}

// GetOutputPath returns the path for final output files (merged, polished, summary).
// Examples:
//
//	GetOutputPath("meeting123", "merged_all.txt")
//	GetOutputPath("meeting123", "polish_all.md")
//	GetOutputPath("meeting123", "meeting_summary.md")
func (pm *PathManager) GetOutputPath(meetingID string, filename string) string {
	return filepath.Join(pm.GetMeetingDir(meetingID), filename)
}

// GetChunkBasename generates the base name for chunk-related files.
// Example: GetChunkBasename(0) -> "chunk_0000"
//
//	GetChunkBasename(15) -> "chunk_0015"
func (pm *PathManager) GetChunkBasename(chunkIndex int) string {
	return fmt.Sprintf("chunk_%04d", chunkIndex)
}

// GetChunkAudioPath returns the full path for a chunk's audio file.
// Example: GetChunkAudioPath("meeting123", 0, "wav")
//
//	-> "/data/meetings/meeting123/chunk_0000.wav"
func (pm *PathManager) GetChunkAudioPath(meetingID string, chunkIndex int, ext string) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s.%s", basename, ext)
	return pm.GetAudioPath(meetingID, filename)
}

// GetChunkTranscriptPath returns the path for a chunk's transcript.
// Example: GetChunkTranscriptPath("meeting123", 5)
//
//	-> "/data/meetings/meeting123/chunk_0005_merged.txt"
func (pm *PathManager) GetChunkTranscriptPath(meetingID string, chunkIndex int) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s_merged.txt", basename)
	return pm.GetTranscriptPath(meetingID, filename)
}

// GetChunkSegmentsPath returns the path for a chunk's diarization segments.
// Example: GetChunkSegmentsPath("meeting123", 5)
//
//	-> "/data/meetings/meeting123/chunk_0005_segments.json"
func (pm *PathManager) GetChunkSegmentsPath(meetingID string, chunkIndex int) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s_segments.json", basename)
	return pm.GetDiarizationPath(meetingID, filename)
}

// GetChunkSpeakersPath returns the path for a chunk's speaker labels.
// Example: GetChunkSpeakersPath("meeting123", 5)
//
//	-> "/data/meetings/meeting123/chunk_0005_speakers.json"
func (pm *PathManager) GetChunkSpeakersPath(meetingID string, chunkIndex int) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s_speakers.json", basename)
	return pm.GetDiarizationPath(meetingID, filename)
}

// GetChunkSpeakersMappedPath returns the path for a chunk's mapped speaker labels.
// Example: GetChunkSpeakersMappedPath("meeting123", 5)
//
//	-> "/data/meetings/meeting123/chunk_0005_speakers_mapped.json"
func (pm *PathManager) GetChunkSpeakersMappedPath(meetingID string, chunkIndex int) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s_speakers_mapped.json", basename)
	return pm.GetDiarizationPath(meetingID, filename)
}

// GetChunkEmbeddingsPath returns the path for a chunk's speaker embeddings.
// Example: GetChunkEmbeddingsPath("meeting123", 5)
//
//	-> "/data/meetings/meeting123/chunk_0005_embeddings.json"
func (pm *PathManager) GetChunkEmbeddingsPath(meetingID string, chunkIndex int) string {
	basename := pm.GetChunkBasename(chunkIndex)
	filename := fmt.Sprintf("%s_embeddings.json", basename)
	return pm.GetEmbeddingsPath(meetingID, filename)
}

// ValidatePath checks if a path is within the shared volume and doesn't contain dangerous patterns.
// Returns error if the path is invalid or potentially malicious.
func (pm *PathManager) ValidatePath(path string) error {
	// 1. Path must be within base directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	absBaseDir, err := filepath.Abs(pm.baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %w", err)
	}

	if !strings.HasPrefix(absPath, absBaseDir) {
		return fmt.Errorf("path %s is outside shared volume (%s)", path, pm.baseDir)
	}

	// 2. Prohibit path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains dangerous characters '..'")
	}

	// 3. Prohibit access to system directories
	dangerousPrefixes := []string{"/etc", "/sys", "/proc", "/dev"}
	for _, prefix := range dangerousPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return fmt.Errorf("access to system directory %s is forbidden", prefix)
		}
	}

	// 4. Prohibit symbolic links (optional security measure)
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not allowed")
	}

	return nil
}

// EnsureMeetingDir creates the meeting directory if it doesn't exist.
// Returns the created directory path and any error.
func (pm *PathManager) EnsureMeetingDir(meetingID string) (string, error) {
	dir := pm.GetMeetingDir(meetingID)

	// Create directory with permission 0755
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create meeting directory: %w", err)
	}

	return dir, nil
}
