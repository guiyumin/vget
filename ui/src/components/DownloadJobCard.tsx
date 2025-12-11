import type { Job, JobStatus } from "../utils/apis";
import type { UITranslations } from "../utils/translations";

interface DownloadJobCardProps {
  job: Job;
  onCancel: () => void;
  onClear: () => void;
  t: UITranslations;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

export function DownloadJobCard({ job, onCancel, onClear, t }: DownloadJobCardProps) {
  const canCancel = job.status === "queued" || job.status === "downloading";
  const canClear = job.status === "completed" || job.status === "failed" || job.status === "cancelled";

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
          {canClear && (
            <button className="clear-btn" onClick={onClear}>
              {t.clear_history}
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
              className={`progress-fill ${job.total <= 0 ? "indeterminate" : ""}`}
              style={{ width: job.total > 0 ? `${job.progress}%` : "100%" }}
            />
          </div>
          <span className="progress-text">
            {job.total > 0
              ? `${job.progress.toFixed(1)}%`
              : formatBytes(job.downloaded)}
          </span>
        </div>
      )}
      {job.status === "failed" && job.error && (
        <div className="error-message">{job.error}</div>
      )}
    </div>
  );
}
