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
	"github.com/guiyumin/vget/internal/core/ai/output"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
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
	StepMerge        StepKey = "merge"
	StepSummarize    StepKey = "summarize"
)

// Step weights for overall progress calculation
var stepWeights = map[StepKey]float64{
	StepExtractAudio: 0.05,
	StepCompress:     0.10,
	StepChunk:        0.05,
	StepTranscribe:   0.50,
	StepMerge:        0.10,
	StepSummarize:    0.20,
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
	useLocalASR         bool               `json:"-"`
	audioLanguage       string             `json:"-"`
	summaryLanguage     string             `json:"-"`
	outputFormat        string             `json:"-"` // md, srt, vtt
}

// AIJobRequest is the request to start an AI processing job
type AIJobRequest struct {
	FilePath           string `json:"file_path"`
	Account            string `json:"account"`             // Cloud account label (optional for local models)
	TranscriptionModel string `json:"transcription_model"` // Model name (e.g., "whisper-medium", "whisper-1")
	SummarizationModel string `json:"summarization_model"`
	PIN                string `json:"pin"`
	IncludeSummary     bool   `json:"include_summary"`
	AudioLanguage      string `json:"audio_language"`   // Language of the audio (e.g., "zh", "en")
	OutputFormat       string `json:"output_format"`    // Output format: "md", "srt", "vtt" (default: "md")
	SummaryLanguage    string `json:"summary_language"` // Language for the summary output (e.g., "zh", "en")
}

// isLocalModel returns true if the model is a local ASR model (whisper.cpp or sherpa-onnx)
func isLocalModel(model string) bool {
	// Local models: whisper-small, whisper-medium, whisper-turbo, parakeet-*
	// Cloud models: whisper-1 (OpenAI API)
	if model == "" {
		return false
	}
	if model == "whisper-1" {
		return false // OpenAI's cloud model
	}
	if strings.HasPrefix(model, "whisper-") || strings.HasPrefix(model, "parakeet") {
		return true
	}
	return false
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

	// Default output format to markdown
	outputFormat := req.OutputFormat
	if outputFormat == "" {
		outputFormat = "md"
	}

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
		useLocalASR:        isLocalModel(req.TranscriptionModel),
		audioLanguage:      req.AudioLanguage,
		summaryLanguage:    req.SummaryLanguage,
		outputFormat:       outputFormat,
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

	// Get translations for current language
	t := i18n.T(q.cfg.Language)

	steps := []AIJobStep{
		{Key: StepExtractAudio, Name: t.UI.AIStepExtractAudio, Status: StepStatusPending},
		{Key: StepCompress, Name: t.UI.AIStepCompressAudio, Status: StepStatusPending},
		{Key: StepChunk, Name: t.UI.AIStepChunkAudio, Status: StepStatusPending},
		{Key: StepTranscribe, Name: t.UI.AIStepTranscribe, Status: StepStatusPending},
		{Key: StepMerge, Name: t.UI.AIStepMerge, Status: StepStatusPending},
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
			Name:     t.UI.AIStepSummarize,
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

	// Create progress callback
	progressFn := func(stepKey StepKey, progress float64, detail string) {
		q.updateStepProgress(job.ID, stepKey, progress, detail)
	}

	var pipeline *ai.Pipeline
	var err error

	if job.useLocalASR {
		// Use local transcription (sherpa-onnx/whisper.cpp)
		// For summarization, we can optionally use a cloud account
		var summarizationAccount *config.AIAccount
		if job.includeSummary && job.account != "" {
			summarizationAccount = cfg.AI.GetAccount(job.account)
			if summarizationAccount == nil {
				q.failJob(job.ID, fmt.Sprintf("AI account '%s' not found for summarization", job.account))
				return
			}
		}
		// Use the model and language from the request, falling back to config if not specified
		localASRCfg := cfg.AI.LocalASR
		if job.transcriptionModel != "" {
			localASRCfg.Model = job.transcriptionModel
		}
		if job.audioLanguage != "" {
			localASRCfg.Language = job.audioLanguage
		}
		pipeline, err = ai.NewLocalPipeline(localASRCfg, summarizationAccount, job.summarizationModel, job.pin)
	} else {
		// Use cloud transcription (OpenAI Whisper API)
		account := cfg.AI.GetAccount(job.account)
		if account == nil {
			q.failJob(job.ID, fmt.Sprintf("AI account '%s' not found", job.account))
			return
		}
		pipeline, err = ai.NewPipelineWithAccount(account, job.transcriptionModel, job.summarizationModel, job.pin)
	}

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

	// Get translations for current language
	t := i18n.T(q.cfg.Language)

	// Check if transcription is already done (resume capability)
	transcriptionDone := q.isStepCompleted(job.ID, StepTranscribe)

	// Determine output paths
	basePath := strings.TrimSuffix(job.FilePath, filepath.Ext(job.FilePath))
	transcriptPath := basePath + ".transcript.md"
	summaryPath := basePath + ".summary.md"

	var transcriptText string

	if transcriptionDone {
		// Resume: Load existing transcript
		progressFn(StepExtractAudio, 100, t.UI.AIDetailTranscriptionComplete)
		progressFn(StepCompress, 100, t.UI.AIDetailTranscriptionComplete)
		progressFn(StepChunk, 100, t.UI.AIDetailTranscriptionComplete)
		progressFn(StepTranscribe, 100, t.UI.AIDetailTranscriptionComplete)
		progressFn(StepMerge, 100, t.UI.AIDetailTranscriptionComplete)

		// Read existing transcript
		data, err := readTranscriptText(transcriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read existing transcript: %w", err)
		}
		transcriptText = data
		result.TranscriptPath = transcriptPath
	} else {
		// Determine file type
		ext := strings.ToLower(filepath.Ext(job.FilePath))
		isVideo := ext == ".mp4" || ext == ".mkv" || ext == ".webm" || ext == ".avi" || ext == ".mov"

		// Step 0: Extract audio (if video)
		if isVideo {
			q.startStep(job.ID, StepExtractAudio)
			progressFn(StepExtractAudio, 0, "")
			// Note: Actual extraction happens in chunker, we track it here
			progressFn(StepExtractAudio, 100, t.UI.AIDetailAudioReady)
			q.completeStep(job.ID, StepExtractAudio)
		} else {
			q.skipStep(job.ID, StepExtractAudio, t.UI.AIDetailAlreadyAudio)
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
			transcriptText = pipelineResult.Transcript.RawText

			// Write transcript in requested output format (if not md)
			if job.outputFormat != "" && job.outputFormat != "md" {
				outputPath, err := output.WriteTranscriptWithFormat(basePath, job.FilePath, job.outputFormat, pipelineResult.Transcript)
				if err != nil {
					return nil, fmt.Errorf("failed to write transcript in %s format: %w", job.outputFormat, err)
				}
				result.TranscriptPath = outputPath
			}
		}
	}

	// Check for cancellation before summarization
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Summary step (with resume support)
	if job.includeSummary {
		summaryDone := q.isStepCompleted(job.ID, StepSummarize)

		if summaryDone {
			// Resume: Load existing summary
			progressFn(StepSummarize, 100, t.UI.AIDetailSummaryGenerated)
			result.SummaryPath = summaryPath
		} else {
			// Run summarization
			q.startStep(job.ID, StepSummarize)
			progressFn(StepSummarize, 0, "")

			// Run summarization through pipeline
			summaryResult, err := pipeline.SummarizeText(ctx, transcriptText, job.FilePath)
			if err != nil {
				return nil, fmt.Errorf("summarization failed: %w", err)
			}

			progressFn(StepSummarize, 100, t.UI.AIDetailSummaryGenerated)
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

	// Extract text content from the transcript markdown
	// Skip metadata lines and extract the actual transcript text
	lines := strings.Split(content, "\n")
	var textLines []string
	inContent := false

	for _, line := range lines {
		// Start collecting after the separator
		if line == "---" {
			inContent = true
			continue
		}
		// Skip if we haven't passed the header yet
		if !inContent {
			continue
		}
		// Collect non-empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			textLines = append(textLines, trimmed)
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
