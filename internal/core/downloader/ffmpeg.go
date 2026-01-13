package downloader

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Log ffmpeg version for debugging
	versionCmd := exec.Command("ffmpeg", "-version")
	versionOut, _ := versionCmd.Output()
	versionLine := strings.Split(string(versionOut), "\n")[0]
	log.Printf("[ffmpeg] version: %s", versionLine)

	// Check input files and log their sizes
	videoInfo, err := os.Stat(videoPath)
	if err != nil {
		log.Printf("[ffmpeg] ERROR: video file not found: %s", videoPath)
		return fmt.Errorf("video file not found: %w", err)
	}
	audioInfo, err := os.Stat(audioPath)
	if err != nil {
		log.Printf("[ffmpeg] ERROR: audio file not found: %s", audioPath)
		return fmt.Errorf("audio file not found: %w", err)
	}

	log.Printf("[ffmpeg] input video: %s (%d bytes)", videoPath, videoInfo.Size())
	log.Printf("[ffmpeg] input audio: %s (%d bytes)", audioPath, audioInfo.Size())
	log.Printf("[ffmpeg] output path: %s", outputPath)

	// Run ffmpeg with stream copy (fast, no re-encoding)
	// -threads 1: single thread for stability in containers
	// -y: overwrite output file without asking
	// -f mp4: explicit output format
	// -map 0:v -map 1:a: take video from first input, audio from second
	args := []string{
		"-threads", "1",
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v",
		"-map", "1:a",
		"-c", "copy",
		"-f", "mp4",
		"-y",
		outputPath,
	}
	log.Printf("[ffmpeg] command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("[ffmpeg] ERROR: merge failed: %v", err)
		log.Printf("[ffmpeg] output:\n%s", string(output))
		return fmt.Errorf("ffmpeg merge failed: %w\nOutput: %s", err, string(output))
	}

	// Check output file
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		log.Printf("[ffmpeg] ERROR: output file not created: %s", outputPath)
		return fmt.Errorf("output file not created: %w", err)
	}

	log.Printf("[ffmpeg] output file: %s (%d bytes)", outputPath, outputInfo.Size())

	// Warn if output is suspiciously small
	inputTotal := videoInfo.Size() + audioInfo.Size()
	if outputInfo.Size() < 1024 || outputInfo.Size() < inputTotal/10 {
		log.Printf("[ffmpeg] WARNING: output file is suspiciously small (%d bytes from %d bytes input)", outputInfo.Size(), inputTotal)
		log.Printf("[ffmpeg] ffmpeg output:\n%s", string(output))
	} else {
		log.Printf("[ffmpeg] merge successful")
	}

	// Delete original files if requested
	if deleteOriginals {
		if err := os.Remove(videoPath); err != nil {
			log.Printf("[ffmpeg] warning: could not remove video file: %v", err)
		}
		if err := os.Remove(audioPath); err != nil {
			log.Printf("[ffmpeg] warning: could not remove audio file: %v", err)
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
