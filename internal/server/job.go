package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// JobStatus represents the current state of a download job
type JobStatus string

const (
	JobStatusQueued      JobStatus = "queued"
	JobStatusDownloading JobStatus = "downloading"
	JobStatusCompleted   JobStatus = "completed"
	JobStatusFailed      JobStatus = "failed"
	JobStatusCancelled   JobStatus = "cancelled"
)

// Job represents a download job
type Job struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Filename   string    `json:"filename,omitempty"`
	Status     JobStatus `json:"status"`
	Progress   float64   `json:"progress"`
	Downloaded int64     `json:"downloaded"` // bytes downloaded
	Total      int64     `json:"total"`      // total bytes (-1 if unknown)
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Internal fields (not serialized)
	cancel context.CancelFunc `json:"-"`
	ctx    context.Context    `json:"-"`
}

// JobQueue manages download jobs with a worker pool
type JobQueue struct {
	jobs          map[string]*Job
	mu            sync.RWMutex
	queue         chan *Job
	maxConcurrent int
	outputDir     string
	downloadFn    DownloadFunc
	wg            sync.WaitGroup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// DownloadFunc is the function signature for downloading a URL
// It receives the job context, URL, output path, and a progress callback
type DownloadFunc func(ctx context.Context, url, outputPath string, progressFn func(downloaded, total int64)) error

// NewJobQueue creates a new job queue with the specified concurrency
func NewJobQueue(maxConcurrent int, outputDir string, downloadFn DownloadFunc) *JobQueue {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	jq := &JobQueue{
		jobs:          make(map[string]*Job),
		queue:         make(chan *Job, 100),
		maxConcurrent: maxConcurrent,
		outputDir:     outputDir,
		downloadFn:    downloadFn,
		stopCleanup:   make(chan struct{}),
	}

	return jq
}

// Start begins the worker pool and cleanup routine
func (jq *JobQueue) Start() {
	// Start workers
	for i := 0; i < jq.maxConcurrent; i++ {
		jq.wg.Add(1)
		go jq.worker()
	}

	// Start cleanup routine (every 10 minutes, remove jobs older than 1 hour)
	jq.cleanupTicker = time.NewTicker(10 * time.Minute)
	go jq.cleanupLoop()
}

// Stop gracefully shuts down the job queue
func (jq *JobQueue) Stop() {
	close(jq.queue)
	close(jq.stopCleanup)
	if jq.cleanupTicker != nil {
		jq.cleanupTicker.Stop()
	}
	jq.wg.Wait()
}

func (jq *JobQueue) worker() {
	defer jq.wg.Done()

	for job := range jq.queue {
		jq.processJob(job)
	}
}

func (jq *JobQueue) processJob(job *Job) {
	jq.updateJobStatus(job.ID, JobStatusDownloading, 0, "")

	// Create progress callback
	progressFn := func(downloaded, total int64) {
		jq.updateJobProgressBytes(job.ID, downloaded, total)
	}

	// Execute download
	err := jq.downloadFn(job.ctx, job.URL, job.Filename, progressFn)

	if err != nil {
		if job.ctx.Err() == context.Canceled {
			jq.updateJobStatus(job.ID, JobStatusCancelled, 0, "cancelled by user")
		} else {
			jq.updateJobStatus(job.ID, JobStatusFailed, 0, err.Error())
		}
		return
	}

	jq.updateJobStatus(job.ID, JobStatusCompleted, 100, "")
}

func (jq *JobQueue) cleanupLoop() {
	for {
		select {
		case <-jq.cleanupTicker.C:
			jq.cleanupOldJobs()
		case <-jq.stopCleanup:
			return
		}
	}
}

func (jq *JobQueue) cleanupOldJobs() {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for id, job := range jq.jobs {
		// Only cleanup completed, failed, or cancelled jobs older than 1 hour
		if (job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled) &&
			job.UpdatedAt.Before(cutoff) {
			delete(jq.jobs, id)
		}
	}
}

// ClearHistory removes all completed, failed, and cancelled jobs
func (jq *JobQueue) ClearHistory() int {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	count := 0
	for id, job := range jq.jobs {
		if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
			delete(jq.jobs, id)
			count++
		}
	}
	return count
}

// RemoveJob removes a single completed, failed, or cancelled job by ID
func (jq *JobQueue) RemoveJob(id string) bool {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	job, ok := jq.jobs[id]
	if !ok {
		return false
	}

	// Can only remove completed, failed, or cancelled jobs
	if job.Status != JobStatusCompleted && job.Status != JobStatusFailed && job.Status != JobStatusCancelled {
		return false
	}

	delete(jq.jobs, id)
	return true
}

// AddJob creates and queues a new download job
func (jq *JobQueue) AddJob(url, filename string) (*Job, error) {
	id, err := generateJobID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate job ID: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	job := &Job{
		ID:        id,
		URL:       url,
		Filename:  filename,
		Status:    JobStatusQueued,
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	jq.mu.Lock()
	jq.jobs[id] = job
	jq.mu.Unlock()

	// Queue the job (non-blocking with buffered channel)
	select {
	case jq.queue <- job:
		return job, nil
	default:
		// Queue is full
		jq.mu.Lock()
		delete(jq.jobs, id)
		jq.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("job queue is full")
	}
}

// GetJob returns a job by ID
func (jq *JobQueue) GetJob(id string) *Job {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	if job, ok := jq.jobs[id]; ok {
		// Return a copy to avoid race conditions
		jobCopy := *job
		return &jobCopy
	}
	return nil
}

// GetAllJobs returns all jobs
func (jq *JobQueue) GetAllJobs() []*Job {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	jobs := make([]*Job, 0, len(jq.jobs))
	for _, job := range jq.jobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs
}

// CancelJob cancels a job by ID
func (jq *JobQueue) CancelJob(id string) bool {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	job, ok := jq.jobs[id]
	if !ok {
		return false
	}

	// Can only cancel queued or downloading jobs
	if job.Status != JobStatusQueued && job.Status != JobStatusDownloading {
		return false
	}

	job.cancel()
	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()
	return true
}

func (jq *JobQueue) updateJobStatus(id string, status JobStatus, progress float64, errMsg string) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if job, ok := jq.jobs[id]; ok {
		job.Status = status
		if progress > 0 {
			job.Progress = progress
		}
		if errMsg != "" {
			job.Error = errMsg
		}
		job.UpdatedAt = time.Now()
	}
}

func (jq *JobQueue) updateJobProgressBytes(id string, downloaded, total int64) {
	jq.mu.Lock()
	defer jq.mu.Unlock()

	if job, ok := jq.jobs[id]; ok {
		job.Downloaded = downloaded
		job.Total = total
		if total > 0 {
			job.Progress = float64(downloaded) / float64(total) * 100
		}
		job.UpdatedAt = time.Now()
	}
}

func generateJobID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
