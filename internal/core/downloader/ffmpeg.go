package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// FFmpegAvailable checks if ffmpeg is installed and available in PATH
func FFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// MergeVideoAudio merges separate video and audio files into a single output file using ffmpeg.
// Uses stream copy (-c copy) for fast merging without re-encoding.
// If deleteOriginals is true, removes the source files after successful merge.
// Returns the path to the merged file.
func MergeVideoAudio(videoPath, audioPath, outputPath string, deleteOriginals bool) error {
	if !FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	// Run ffmpeg with stream copy (fast, no re-encoding)
	// -y: overwrite output file without asking
	cmd := exec.Command("ffmpeg",
		"-threads", "1",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg merge failed: %w\nOutput: %s", err, string(output))
	}

	// Delete original files if requested
	if deleteOriginals {
		if err := os.Remove(videoPath); err != nil {
			// Log but don't fail - the merged file was created successfully
			fmt.Printf("Warning: could not remove video file: %v\n", err)
		}
		if err := os.Remove(audioPath); err != nil {
			fmt.Printf("Warning: could not remove audio file: %v\n", err)
		}
	}

	return nil
}

// MergeVideoAudioKeepOriginals merges video and audio into a new file with "(merged)" prefix.
// Original video and audio files are kept.
// Returns the path to the merged file.
func MergeVideoAudioKeepOriginals(videoPath, audioPath string) (string, error) {
	if !FFmpegAvailable() {
		return "", fmt.Errorf("ffmpeg not found in PATH")
	}

	// Build merged output path with "(merged)" prefix
	dir := filepath.Dir(videoPath)
	filename := filepath.Base(videoPath)
	mergedPath := filepath.Join(dir, "(merged)"+filename)

	// Merge to new file, keep originals
	if err := MergeVideoAudio(videoPath, audioPath, mergedPath, false); err != nil {
		return "", err
	}

	return mergedPath, nil
}
