package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/guiyumin/vget/internal/core/ai"
	"github.com/guiyumin/vget/internal/core/config"
)

// AIJobStatus represents the current state of an AI processing job
type AIJobStatus string

const (
	AIJobStatusQueued     AIJobStatus = "queued"
	AIJobStatusProcessing AIJobStatus = "processing"
	AIJobStatusCompleted  AIJobStatus = "completed"
	AIJobStatusFailed     AIJobStatus = "failed"
	AIJobStatusCancelled  AIJobStatus = "cancelled"
)

// StepStatus represents the state of a processing step
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusInProgress StepStatus = "in_progress"
	StepStatusCompleted  StepStatus = "completed"
	StepStatusSkipped    StepStatus = "skipped"
	StepStatusFailed     StepStatus = "failed"
)

// StepKey identifies each processing step
type StepKey string

const (
	StepExtractAudio StepKey = "extract_audio"
	StepCompress     StepKey = "compress_audio"
	StepChunk        StepKey = "chunk_audio"
	StepTranscribe   StepKey = "transcribe"
	StepCleanup      StepKey = "cleanup"
	StepMerge        StepKey = "merge"
	StepSummarize    StepKey = "summarize"
)

// Step weights for overall progress calculation
var stepWeights = map[StepKey]float64{
	StepExtractAudio: 0.05,
	StepCompress:     0.10,
	StepChunk:        0.05,
	StepTranscribe:   0.45,
	StepCleanup:      0.15,
	StepMerge:        0.05,
	StepSummarize:    0.15,
}

// AIJobStep represents a single processing step
type AIJobStep struct {
	Key        StepKey    `json:"key"`
	Name       string     `json:"name"`
	Status     StepStatus `json:"status"`
	Progress   float64    `json:"progress"` // 0-100
	Detail     string     `json:"detail,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// AIJobResult contains the outputs of the processing
type AIJobResult struct {
	TranscriptPath string `json:"transcript_path,omitempty"`
	SummaryPath    string `json:"summary_path,omitempty"`
	RawText        string `json:"raw_text,omitempty"`
	CleanedText    string `json:"cleaned_text,omitempty"`
	Summary        string `json:"summary,omitempty"`
}

// AIJob represents an AI processing job
type AIJob struct {
	ID              string       `json:"id"`
	FilePath        string       `json:"file_path"`
	FileName        string       `json:"file_name"`
	Status          AIJobStatus  `json:"status"`
	CurrentStep     StepKey      `json:"current_step"`
	Steps           []AIJobStep  `json:"steps"`
	OverallProgress float64      `json:"overall_progress"` // 0-100
	Result          *AIJobResult `json:"result,omitempty"`
	Error           string       `json:"error,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`

	// Internal fields (not serialized)
	ctx                 context.Context    `json:"-"`
	cancel              context.CancelFunc `json:"-"`
	account             string             `json:"-"`
	transcriptionModel  string             `json:"-"`
	summarizationModel  string             `json:"-"`
	pin                 string             `json:"-"`
	includeSummary      bool               `json:"-"`
}

// AIJobRequest is the request to start an AI processing job
type AIJobRequest struct {
	FilePath           string `json:"file_path"`
	Account            string `json:"account"`
	TranscriptionModel string `json:"transcription_model"`
	SummarizationModel string `json:"summarization_model"`
	PIN                string `json:"pin"`
	IncludeSummary     bool   `json:"include_summary"`
}

// AIJobQueue manages AI processing jobs with a worker pool
type AIJobQueue struct {
	jobs          map[string]*AIJob
	mu            sync.RWMutex
	queue         chan *AIJob
	maxConcurrent int
	outputDir     string
	cfg           *config.Config
	wg            sync.WaitGroup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewAIJobQueue creates a new AI job queue
func NewAIJobQueue(maxConcurrent int, outputDir string, cfg *config.Config) *AIJobQueue {
	if maxConcurrent <= 0 {
		maxConcurrent = 2 // Default to 2 concurrent AI jobs
	}

	return &AIJobQueue{
		jobs:          make(map[string]*AIJob),
		queue:         make(chan *AIJob, 50),
		maxConcurrent: maxConcurrent,
		outputDir:     outputDir,
		cfg:           cfg,
		stopCleanup:   make(chan struct{}),
	}
}

// Start begins the worker pool and cleanup routine
func (q *AIJobQueue) Start() {
	for i := 0; i < q.maxConcurrent; i++ {
		q.wg.Add(1)
		go q.worker()
	}

	// Cleanup every 30 minutes, remove jobs older than 2 hours
	q.cleanupTicker = time.NewTicker(30 * time.Minute)
	go q.cleanupLoop()
}

// Stop gracefully shuts down the job queue
func (q *AIJobQueue) Stop() {
	close(q.queue)
	close(q.stopCleanup)
	if q.cleanupTicker != nil {
		q.cleanupTicker.Stop()
	}
	q.wg.Wait()
}

func (q *AIJobQueue) worker() {
	defer q.wg.Done()

	for job := range q.queue {
		q.processJob(job)
	}
}

func (q *AIJobQueue) cleanupLoop() {
	for {
		select {
		case <-q.cleanupTicker.C:
			q.cleanupOldJobs()
		case <-q.stopCleanup:
			return
		}
	}
}

func (q *AIJobQueue) cleanupOldJobs() {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().Add(-2 * time.Hour)
	for id, job := range q.jobs {
		if (job.Status == AIJobStatusCompleted || job.Status == AIJobStatusFailed || job.Status == AIJobStatusCancelled) &&
			job.UpdatedAt.Before(cutoff) {
			delete(q.jobs, id)
		}
	}
}

// AddJob creates and queues a new AI processing job
func (q *AIJobQueue) AddJob(req AIJobRequest) (*AIJob, error) {
	id, err := generateAIJobID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate job ID: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize steps based on file type and options
	steps := q.initializeSteps(req.FilePath, req.IncludeSummary)

	job := &AIJob{
		ID:                 id,
		FilePath:           req.FilePath,
		FileName:           filepath.Base(req.FilePath),
		Status:             AIJobStatusQueued,
		Steps:              steps,
		OverallProgress:    0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		ctx:                ctx,
		cancel:             cancel,
		account:            req.Account,
		transcriptionModel: req.TranscriptionModel,
		summarizationModel: req.SummarizationModel,
		pin:                req.PIN,
		includeSummary:     req.IncludeSummary,
	}

	// Calculate initial overall progress (for resume capability)
	q.updateOverallProgress(job)

	q.mu.Lock()
	q.jobs[id] = job
	q.mu.Unlock()

	// Queue the job
	select {
	case q.queue <- job:
		return job, nil
	default:
		q.mu.Lock()
		delete(q.jobs, id)
		q.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("AI job queue is full")
	}
}

// initializeSteps creates the step list based on options and existing artifacts
func (q *AIJobQueue) initializeSteps(filePath string, includeSummary bool) []AIJobStep {
	// Check for existing artifacts to enable resume capability
	basePath := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	transcriptPath := basePath + ".transcript.md"
	summaryPath := basePath + ".summary.md"

	hasTranscript := fileExists(transcriptPath)
	hasSummary := fileExists(summaryPath)

	steps := []AIJobStep{
		{Key: StepExtractAudio, Name: "Extract Audio", Status: StepStatusPending},
		{Key: StepCompress, Name: "Compress Audio", Status: StepStatusPending},
		{Key: StepChunk, Name: "Chunk Audio", Status: StepStatusPending},
		{Key: StepTranscribe, Name: "Transcribe", Status: StepStatusPending},
		{Key: StepMerge, Name: "Merge Chunks", Status: StepStatusPending},
		{Key: StepCleanup, Name: "Clean Transcript", Status: StepStatusPending},
	}

	// If transcript exists, mark transcription steps as completed (resume point)
	if hasTranscript {
		for i := range steps {
			steps[i].Status = StepStatusCompleted
			steps[i].Progress = 100
		}
	}

	if includeSummary {
		summaryStatus := StepStatusPending
		if hasSummary {
			summaryStatus = StepStatusCompleted
		}
		steps = append(steps, AIJobStep{
			Key:      StepSummarize,
			Name:     "Generate Summary",
			Status:   summaryStatus,
			Progress: func() float64 { if hasSummary { return 100 }; return 0 }(),
		})
	}

	return steps
}

// GetJob returns a job by ID
func (q *AIJobQueue) GetJob(id string) *AIJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if job, ok := q.jobs[id]; ok {
		return q.copyJob(job)
	}
	return nil
}

// GetAllJobs returns all jobs
func (q *AIJobQueue) GetAllJobs() []*AIJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	jobs := make([]*AIJob, 0, len(q.jobs))
	for _, job := range q.jobs {
		jobs = append(jobs, q.copyJob(job))
	}
	return jobs
}

// copyJob creates a safe copy of a job for external use
func (q *AIJobQueue) copyJob(job *AIJob) *AIJob {
	jobCopy := *job
	jobCopy.ctx = nil
	jobCopy.cancel = nil
	jobCopy.pin = "" // Never expose PIN

	// Deep copy steps
	jobCopy.Steps = make([]AIJobStep, len(job.Steps))
	copy(jobCopy.Steps, job.Steps)

	// Deep copy result if present
	if job.Result != nil {
		resultCopy := *job.Result
		jobCopy.Result = &resultCopy
	}

	return &jobCopy
}

// CancelJob cancels a job by ID
func (q *AIJobQueue) CancelJob(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, ok := q.jobs[id]
	if !ok {
		return false
	}

	if job.Status != AIJobStatusQueued && job.Status != AIJobStatusProcessing {
		return false
	}

	job.cancel()
	job.Status = AIJobStatusCancelled
	job.UpdatedAt = time.Now()
	return true
}

// ClearHistory removes all completed, failed, and cancelled jobs
func (q *AIJobQueue) ClearHistory() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := 0
	for id, job := range q.jobs {
		if job.Status == AIJobStatusCompleted || job.Status == AIJobStatusFailed || job.Status == AIJobStatusCancelled {
			delete(q.jobs, id)
			count++
		}
	}
	return count
}

// processJob executes the AI pipeline for a job
func (q *AIJobQueue) processJob(job *AIJob) {
	q.updateJobStatus(job.ID, AIJobStatusProcessing)

	// Reload config to get fresh AI accounts
	cfg := config.LoadOrDefault()

	// Get the AI account
	account := cfg.AI.GetAccount(job.account)
	if account == nil {
		q.failJob(job.ID, fmt.Sprintf("AI account '%s' not found", job.account))
		return
	}

	// Create progress callback
	progressFn := func(stepKey StepKey, progress float64, detail string) {
		q.updateStepProgress(job.ID, stepKey, progress, detail)
	}

	// Create pipeline with progress support
	pipeline, err := ai.NewPipelineWithAccount(account, job.transcriptionModel, job.summarizationModel, job.pin)
	if err != nil {
		q.failJob(job.ID, fmt.Sprintf("Failed to create pipeline: %v", err))
		return
	}

	// Execute with progress callbacks
	result, err := q.executeWithProgress(job.ctx, pipeline, job, progressFn)
	if err != nil {
		if job.ctx.Err() == context.Canceled {
			q.updateJobStatus(job.ID, AIJobStatusCancelled)
		} else {
			q.failJob(job.ID, err.Error())
		}
		return
	}

	// Store result and mark completed
	q.mu.Lock()
	if j, ok := q.jobs[job.ID]; ok {
		j.Result = result
		j.Status = AIJobStatusCompleted
		j.OverallProgress = 100
		j.UpdatedAt = time.Now()

		// Mark all steps as completed
		for i := range j.Steps {
			if j.Steps[i].Status == StepStatusInProgress {
				j.Steps[i].Status = StepStatusCompleted
				j.Steps[i].Progress = 100
				now := time.Now()
				j.Steps[i].FinishedAt = &now
			}
		}
	}
	q.mu.Unlock()
}

// executeWithProgress runs the pipeline with step-by-step progress updates
// It supports resume capability by checking if steps are already completed
func (q *AIJobQueue) executeWithProgress(ctx context.Context, pipeline *ai.Pipeline, job *AIJob, progressFn func(StepKey, float64, string)) (*AIJobResult, error) {
	result := &AIJobResult{}

	// Check if transcription is already done (resume capability)
	transcriptionDone := q.isStepCompleted(job.ID, StepTranscribe)

	// Determine output paths
	basePath := strings.TrimSuffix(job.FilePath, filepath.Ext(job.FilePath))
	transcriptPath := basePath + ".transcript.md"
	summaryPath := basePath + ".summary.md"

	var transcriptText string

	if transcriptionDone {
		// Resume: Load existing transcript
		progressFn(StepExtractAudio, 100, "Skipped (transcript exists)")
		progressFn(StepCompress, 100, "Skipped (transcript exists)")
		progressFn(StepChunk, 100, "Skipped (transcript exists)")
		progressFn(StepTranscribe, 100, "Skipped (transcript exists)")
		progressFn(StepCleanup, 100, "Skipped (transcript exists)")
		progressFn(StepMerge, 100, "Skipped (transcript exists)")

		// Read existing transcript
		data, err := readTranscriptText(transcriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read existing transcript: %w", err)
		}
		transcriptText = data
		result.TranscriptPath = transcriptPath
		result.CleanedText = transcriptText
	} else {
		// Determine file type
		ext := strings.ToLower(filepath.Ext(job.FilePath))
		isVideo := ext == ".mp4" || ext == ".mkv" || ext == ".webm" || ext == ".avi" || ext == ".mov"

		// Step 0: Extract audio (if video)
		if isVideo {
			q.startStep(job.ID, StepExtractAudio)
			progressFn(StepExtractAudio, 0, "Extracting audio from video...")
			// Note: Actual extraction happens in chunker, we track it here
			progressFn(StepExtractAudio, 100, "Audio extracted")
			q.completeStep(job.ID, StepExtractAudio)
		} else {
			q.skipStep(job.ID, StepExtractAudio, "Already audio")
		}

		// Check for cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Steps 1-5: Transcription (handled by pipeline with progress reporting)
		q.startStep(job.ID, StepCompress)

		// Create a progress callback that maps ai.ProgressStep to our StepKey
		pipelineProgressFn := func(step ai.ProgressStep, progress float64, detail string) {
			switch step {
			case ai.ProgressStepCompress:
				progressFn(StepCompress, progress, detail)
				if progress >= 100 {
					q.completeStep(job.ID, StepCompress)
					q.startStep(job.ID, StepChunk)
				}
			case ai.ProgressStepChunk:
				progressFn(StepChunk, progress, detail)
				if progress >= 100 {
					q.completeStep(job.ID, StepChunk)
					q.startStep(job.ID, StepTranscribe)
				} else if progress == 0 {
					// Starting chunk step
					q.completeStep(job.ID, StepCompress)
					q.startStep(job.ID, StepChunk)
				}
			case ai.ProgressStepTranscribe:
				progressFn(StepTranscribe, progress, detail)
				if progress >= 100 {
					q.completeStep(job.ID, StepTranscribe)
					q.startStep(job.ID, StepMerge)
				} else if progress == 0 {
					// Starting transcribe step
					q.startStep(job.ID, StepTranscribe)
				}
			case ai.ProgressStepMerge:
				progressFn(StepMerge, progress, detail)
				if progress >= 100 {
					q.completeStep(job.ID, StepMerge)
					q.startStep(job.ID, StepCleanup)
				}
			case ai.ProgressStepCleanup:
				progressFn(StepCleanup, progress, detail)
				if progress >= 100 {
					q.completeStep(job.ID, StepCleanup)
				}
			}
		}

		// Run the actual pipeline with progress callback
		opts := ai.Options{
			Transcribe: true,
			Summarize:  false, // We'll handle summarization separately for resume support
		}

		pipelineResult, err := pipeline.ProcessWithProgress(ctx, job.FilePath, opts, pipelineProgressFn)
		if err != nil {
			return nil, err
		}

		// Build result
		result.TranscriptPath = pipelineResult.TranscriptPath
		if pipelineResult.Transcript != nil {
			result.RawText = pipelineResult.Transcript.RawText
			result.CleanedText = pipelineResult.Transcript.CleanedText
			// Use cleaned text if available, otherwise raw text
			if pipelineResult.Transcript.CleanedText != "" {
				transcriptText = pipelineResult.Transcript.CleanedText
			} else {
				transcriptText = pipelineResult.Transcript.RawText
			}
		}
	}

	// Check for cancellation before summarization
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Step 6: Summary (with resume support)
	if job.includeSummary {
		summaryDone := q.isStepCompleted(job.ID, StepSummarize)

		if summaryDone {
			// Resume: Load existing summary
			progressFn(StepSummarize, 100, "Skipped (summary exists)")
			result.SummaryPath = summaryPath
		} else {
			// Run summarization
			q.startStep(job.ID, StepSummarize)
			progressFn(StepSummarize, 0, "Generating summary...")

			// Run summarization through pipeline
			summaryResult, err := pipeline.SummarizeText(ctx, transcriptText, job.FilePath)
			if err != nil {
				return nil, fmt.Errorf("summarization failed: %w", err)
			}

			progressFn(StepSummarize, 100, "Summary generated")
			q.completeStep(job.ID, StepSummarize)

			result.SummaryPath = summaryResult.SummaryPath
			result.Summary = summaryResult.Summary
		}
	}

	return result, nil
}

// isStepCompleted checks if a step is already completed
func (q *AIJobQueue) isStepCompleted(jobID string, stepKey StepKey) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if job, ok := q.jobs[jobID]; ok {
		for _, step := range job.Steps {
			if step.Key == stepKey {
				return step.Status == StepStatusCompleted
			}
		}
	}
	return false
}

// readTranscriptText reads the transcript text from a markdown file
func readTranscriptText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)

	// The transcript markdown has sections like "## Transcript" or "## Cleaned Transcript"
	// We want to extract the text content, preferring cleaned if available
	lines := strings.Split(content, "\n")
	var textLines []string
	inTranscript := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## Cleaned Transcript") {
			inTranscript = true
			textLines = nil // Reset to prefer cleaned transcript
			continue
		}
		if strings.HasPrefix(line, "## Transcript") {
			if len(textLines) == 0 { // Only use raw if we haven't found cleaned
				inTranscript = true
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inTranscript = false
			continue
		}
		if inTranscript && strings.TrimSpace(line) != "" {
			textLines = append(textLines, line)
		}
	}

	return strings.Join(textLines, "\n"), nil
}

// Step state management helpers
func (q *AIJobQueue) startStep(jobID string, stepKey StepKey) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		for i := range job.Steps {
			if job.Steps[i].Key == stepKey {
				job.Steps[i].Status = StepStatusInProgress
				job.Steps[i].Progress = 0
				now := time.Now()
				job.Steps[i].StartedAt = &now
				job.CurrentStep = stepKey
				job.UpdatedAt = now
				break
			}
		}
		q.updateOverallProgress(job)
	}
}

func (q *AIJobQueue) completeStep(jobID string, stepKey StepKey) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		for i := range job.Steps {
			if job.Steps[i].Key == stepKey {
				job.Steps[i].Status = StepStatusCompleted
				job.Steps[i].Progress = 100
				now := time.Now()
				job.Steps[i].FinishedAt = &now
				job.UpdatedAt = now
				break
			}
		}
		q.updateOverallProgress(job)
	}
}

func (q *AIJobQueue) skipStep(jobID string, stepKey StepKey, reason string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		for i := range job.Steps {
			if job.Steps[i].Key == stepKey {
				job.Steps[i].Status = StepStatusSkipped
				job.Steps[i].Detail = reason
				now := time.Now()
				job.Steps[i].FinishedAt = &now
				job.UpdatedAt = now
				break
			}
		}
		q.updateOverallProgress(job)
	}
}

func (q *AIJobQueue) updateStepProgress(jobID string, stepKey StepKey, progress float64, detail string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		for i := range job.Steps {
			if job.Steps[i].Key == stepKey {
				job.Steps[i].Progress = progress
				if detail != "" {
					job.Steps[i].Detail = detail
				}
				job.UpdatedAt = time.Now()
				break
			}
		}
		q.updateOverallProgress(job)
	}
}

func (q *AIJobQueue) updateOverallProgress(job *AIJob) {
	totalWeight := 0.0
	completedWeight := 0.0

	for _, step := range job.Steps {
		weight := stepWeights[step.Key]
		totalWeight += weight

		switch step.Status {
		case StepStatusCompleted, StepStatusSkipped:
			completedWeight += weight
		case StepStatusInProgress:
			completedWeight += weight * (step.Progress / 100)
		}
	}

	if totalWeight > 0 {
		job.OverallProgress = (completedWeight / totalWeight) * 100
	}
}

func (q *AIJobQueue) updateJobStatus(jobID string, status AIJobStatus) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		job.Status = status
		job.UpdatedAt = time.Now()
	}
}

func (q *AIJobQueue) failJob(jobID string, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobs[jobID]; ok {
		job.Status = AIJobStatusFailed
		job.Error = errMsg
		job.UpdatedAt = time.Now()

		// Mark current step as failed
		for i := range job.Steps {
			if job.Steps[i].Status == StepStatusInProgress {
				job.Steps[i].Status = StepStatusFailed
				now := time.Now()
				job.Steps[i].FinishedAt = &now
				break
			}
		}
	}
}

func generateAIJobID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ai-" + hex.EncodeToString(bytes), nil
}
