import { useState, useEffect, useCallback } from 'react'
import './App.css'

type JobStatus = 'queued' | 'downloading' | 'completed' | 'failed' | 'cancelled'

interface Job {
  id: string
  url: string
  status: JobStatus
  progress: number
  filename?: string
  error?: string
}

interface ApiResponse<T> {
  code: number
  data: T
  message: string
}

interface HealthData {
  status: string
  version: string
}

interface JobsData {
  jobs: Job[]
}

async function fetchHealth(): Promise<ApiResponse<HealthData>> {
  const res = await fetch('/health')
  return res.json()
}

async function fetchJobs(): Promise<ApiResponse<JobsData>> {
  const res = await fetch('/jobs')
  return res.json()
}

async function postDownload(url: string): Promise<ApiResponse<{ id: string; status: string }>> {
  const res = await fetch('/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url }),
  })
  return res.json()
}

async function deleteJob(id: string): Promise<ApiResponse<{ id: string }>> {
  const res = await fetch(`/jobs/${id}`, { method: 'DELETE' })
  return res.json()
}

function App() {
  const [health, setHealth] = useState<HealthData | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const [healthRes, jobsRes] = await Promise.all([fetchHealth(), fetchJobs()])
      if (healthRes.code === 200) setHealth(healthRes.data)
      if (jobsRes.code === 200) setJobs(jobsRes.data.jobs || [])
    } catch {
      setHealth(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, 1000)
    return () => clearInterval(interval)
  }, [refresh])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!url.trim() || submitting) return

    setSubmitting(true)
    try {
      const res = await postDownload(url.trim())
      if (res.code === 200) {
        setUrl('')
        refresh()
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleCancel = async (id: string) => {
    await deleteJob(id)
    refresh()
  }

  const isConnected = health?.status === 'ok'

  const sortedJobs = [...jobs].sort((a, b) => {
    const order: Record<JobStatus, number> = {
      downloading: 0, queued: 1, completed: 2, failed: 3, cancelled: 4
    }
    return (order[a.status] ?? 5) - (order[b.status] ?? 5)
  })

  return (
    <div className="container">
      <header className="header">
        <div className="header-left">
          <span className={`status-dot ${isConnected ? 'ok' : 'error'}`} />
          <h1>vget server</h1>
        </div>
        <span className="version">{health?.version || '...'}</span>
      </header>

      <form className="download-form" onSubmit={handleSubmit}>
        <input
          type="text"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="Paste URL to download..."
          disabled={!isConnected || submitting}
        />
        <button type="submit" disabled={!isConnected || !url.trim() || submitting}>
          {submitting ? 'Adding...' : 'Download'}
        </button>
      </form>

      <section className="jobs-section">
        <div className="jobs-header">
          <h2>Jobs</h2>
          <span className="count">{jobs.length} total</span>
        </div>

        {loading ? (
          <div className="empty-state">Loading...</div>
        ) : sortedJobs.length === 0 ? (
          <div className="empty-state">
            <p>No downloads yet</p>
            <p className="hint">Paste a URL above to get started</p>
          </div>
        ) : (
          <div className="jobs-list">
            {sortedJobs.map((job) => (
              <JobCard key={job.id} job={job} onCancel={() => handleCancel(job.id)} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function JobCard({ job, onCancel }: { job: Job; onCancel: () => void }) {
  const canCancel = job.status === 'queued' || job.status === 'downloading'

  return (
    <div className="job-card">
      <div className="job-header">
        <code className="job-id">{job.id}</code>
        <div className="job-actions">
          <span className={`status-badge ${job.status}`}>{job.status}</span>
          {canCancel && (
            <button className="cancel-btn" onClick={onCancel}>Cancel</button>
          )}
        </div>
      </div>
      <p className="job-url">{job.url}</p>
      {job.filename && <p className="job-filename">{job.filename}</p>}
      {job.status === 'downloading' && (
        <div className="progress-container">
          <div className="progress-bar">
            <div className="progress-fill" style={{ width: `${job.progress}%` }} />
          </div>
          <span className="progress-text">{job.progress.toFixed(1)}%</span>
        </div>
      )}
      {job.status === 'failed' && job.error && (
        <div className="error-message">{job.error}</div>
      )}
    </div>
  )
}

export default App
