import { useState } from "react";
import clsx from "clsx";
import { useApp } from "../context/AppContext";
import { DownloadJobCard } from "../components/DownloadJobCard";

export function DownloadPage() {
  const {
    isConnected,
    loading,
    jobs,
    outputDir,
    t,
    submitDownload,
    cancelJob,
    clearAllHistory,
    updateOutputDir,
  } = useApp();

  const [url, setUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [editingDir, setEditingDir] = useState(false);
  const [newOutputDir, setNewOutputDir] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || submitting) return;

    setSubmitting(true);
    try {
      const success = await submitDownload(url.trim());
      if (success) {
        setUrl("");
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleEditDir = () => {
    setNewOutputDir(outputDir);
    setEditingDir(true);
  };

  const handleSaveDir = async () => {
    if (!newOutputDir.trim()) return;
    const success = await updateOutputDir(newOutputDir.trim());
    if (success) {
      setEditingDir(false);
    }
  };

  const handleCancelEdit = () => {
    setEditingDir(false);
    setNewOutputDir("");
  };

  // Sort by title (filename or URL) for stable ordering
  const sortedJobs = [...jobs].sort((a, b) => {
    const titleA = a.filename || a.url;
    const titleB = b.filename || b.url;
    return titleA.localeCompare(titleB);
  });

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-4">
      <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-3">
        <span className="text-zinc-700 dark:text-zinc-200 text-sm whitespace-nowrap">
          {t.download_to}
        </span>
        <input
          type="text"
          className={clsx(
            "flex-1 px-3 py-2 border rounded font-mono text-xs sm:text-sm transition-colors focus:outline-none placeholder:text-zinc-400 dark:placeholder:text-zinc-600",
            editingDir
              ? "border-blue-500 bg-zinc-100 dark:bg-zinc-950 text-zinc-900 dark:text-white"
              : "border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 text-zinc-700 dark:text-zinc-200 cursor-default"
          )}
          value={editingDir ? newOutputDir : outputDir}
          onChange={(e) => setNewOutputDir(e.target.value)}
          onKeyDown={(e) => {
            if (editingDir && e.key === "Enter") handleSaveDir();
            if (editingDir && e.key === "Escape") handleCancelEdit();
          }}
          readOnly={!editingDir}
          placeholder="..."
        />
        {editingDir ? (
          <div className="flex gap-2 self-end sm:self-auto">
            <button
              onClick={handleSaveDir}
              className="px-3 py-1.5 border border-green-500 text-green-500 rounded text-xs cursor-pointer whitespace-nowrap hover:bg-green-500 hover:text-white transition-colors"
            >
              {t.save}
            </button>
            <button
              onClick={handleCancelEdit}
              className="px-3 py-1.5 border border-zinc-300 dark:border-zinc-700 text-zinc-500 rounded text-xs cursor-pointer whitespace-nowrap hover:border-zinc-500 hover:text-zinc-900 dark:hover:text-white transition-colors"
            >
              {t.cancel}
            </button>
          </div>
        ) : (
          <button
            onClick={handleEditDir}
            className="px-3 py-1.5 border border-zinc-300 dark:border-zinc-700 text-zinc-500 rounded text-xs cursor-pointer whitespace-nowrap hover:border-blue-500 hover:text-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors self-end sm:self-auto"
            disabled={!isConnected}
          >
            {t.edit}
          </button>
        )}
      </div>

      <form className="flex flex-col sm:flex-row gap-3" onSubmit={handleSubmit}>
        <input
          type="text"
          className="flex-1 px-4 py-3 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-base focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t.paste_url}
          disabled={!isConnected || submitting}
        />
        <button
          type="submit"
          className="px-6 py-3 border-none rounded-lg bg-blue-500 text-white text-base font-medium cursor-pointer hover:bg-blue-600 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 disabled:cursor-not-allowed transition-colors"
          disabled={!isConnected || !url.trim() || submitting}
        >
          {submitting ? t.adding : t.download}
        </button>
      </form>

      <section className="mt-4">
        <div className="flex items-center gap-3 mb-4">
          <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
            {t.jobs}
          </h2>
          <span className="text-zinc-700 dark:text-zinc-200 text-sm">
            {jobs.length} {t.total}
          </span>
          <div className="flex gap-2 ml-auto">
            <button
              className="px-2 py-1 border border-zinc-300 dark:border-zinc-700 rounded bg-transparent text-zinc-500 text-[0.7rem] cursor-pointer transition-colors hover:border-red-500 hover:text-red-500 disabled:opacity-50 disabled:cursor-not-allowed"
              onClick={clearAllHistory}
              disabled={
                !isConnected ||
                !jobs.some(
                  (j) =>
                    j.status === "completed" ||
                    j.status === "failed" ||
                    j.status === "cancelled"
                )
              }
              title={t.clear_all}
            >
              {t.clear_all}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
            Loading...
          </div>
        ) : sortedJobs.length === 0 ? (
          <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
            <p>{t.no_downloads}</p>
            <p className="text-sm mt-2">{t.paste_hint}</p>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {sortedJobs.map((job) => (
              <DownloadJobCard
                key={job.id}
                job={job}
                onCancel={() => cancelJob(job.id)}
                onClear={() => cancelJob(job.id)}
                t={t}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
