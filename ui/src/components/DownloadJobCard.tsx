import type { Job, JobStatus } from "../utils/apis";
import type { UITranslations } from "../utils/translations";

interface DownloadJobCardProps {
  job: Job;
  onCancel: () => void;
  t: UITranslations;
}

export function DownloadJobCard({ job, onCancel, t }: DownloadJobCardProps) {
  const canCancel = job.status === "queued" || job.status === "downloading";

  const statusText: Record<JobStatus, string> = {
    queued: t.queued,
    downloading: t.downloading,
    completed: t.completed,
    failed: t.failed,
    cancelled: t.cancelled,
  };

  return (
    <div className="job-card">
      <div className="job-header">
        <code className="job-id">{job.id}</code>
        <div className="job-actions">
          <span className={`status-badge ${job.status}`}>
            {statusText[job.status]}
          </span>
          {canCancel && (
            <button className="cancel-btn" onClick={onCancel}>
              {t.cancel}
            </button>
          )}
        </div>
      </div>
      <p className="job-url">{job.url}</p>
      {job.filename && <p className="job-filename">{job.filename}</p>}
      {job.status === "downloading" && (
        <div className="progress-container">
          <div className="progress-bar">
            <div
              className="progress-fill"
              style={{ width: `${job.progress}%` }}
            />
          </div>
          <span className="progress-text">{job.progress.toFixed(1)}%</span>
        </div>
      )}
      {job.status === "failed" && job.error && (
        <div className="error-message">{job.error}</div>
      )}
    </div>
  );
}
