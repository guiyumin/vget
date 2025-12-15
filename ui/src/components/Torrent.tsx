import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useApp } from "../context/AppContext";
import { addTorrent } from "../utils/apis";

interface TorrentProps {
  isConnected: boolean;
  torrentEnabled: boolean;
}

export function Torrent({ isConnected, torrentEnabled }: TorrentProps) {
  const { t } = useApp();
  const [magnetUrl, setMagnetUrl] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [result, setResult] = useState<{
    success: boolean;
    message: string;
    name?: string;
  } | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!magnetUrl.trim() || isSubmitting) return;

    setIsSubmitting(true);
    setResult(null);

    try {
      const res = await addTorrent(magnetUrl.trim());
      if (res.code === 200) {
        setResult({
          success: true,
          message: res.data.duplicate
            ? "Torrent already exists"
            : t.torrent_success,
          name: res.data.name,
        });
        setMagnetUrl("");
      } else {
        setResult({
          success: false,
          message: res.message || "Failed to add torrent",
        });
      }
    } catch {
      setResult({
        success: false,
        message: "Network error",
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!torrentEnabled) {
    return (
      <section className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg mb-6 overflow-hidden">
        <div className="flex justify-between items-center px-4 py-3">
          <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
            {t.torrent}
          </h2>
        </div>
        <div className="p-4 border-t border-zinc-300 dark:border-zinc-700">
          <div className="text-sm text-zinc-500 dark:text-zinc-400 mb-3">
            {t.torrent_not_configured}
          </div>
          <Link
            to="/config"
            className="inline-block px-4 py-2 bg-blue-500 text-white text-sm font-medium rounded-md hover:bg-blue-600 transition-colors"
          >
            {t.settings}
          </Link>
        </div>
      </section>
    );
  }

  return (
    <section className="bg-white dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-700 rounded-lg mb-6 overflow-hidden">
      <div className="flex justify-between items-center px-4 py-3">
        <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200">
          {t.torrent}
        </h2>
      </div>
      <div className="p-4 border-t border-zinc-300 dark:border-zinc-700">
        <form className="flex gap-3 flex-wrap" onSubmit={handleSubmit}>
          <input
            type="text"
            value={magnetUrl}
            onChange={(e) => setMagnetUrl(e.target.value)}
            placeholder={t.torrent_hint}
            disabled={!isConnected || isSubmitting}
            className="flex-1 min-w-50 px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-md bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-sm focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
          />
          <button
            type="submit"
            disabled={!isConnected || !magnetUrl.trim() || isSubmitting}
            className="px-4 py-2 border-none rounded-md bg-blue-500 text-white text-sm font-medium cursor-pointer whitespace-nowrap hover:bg-blue-600 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 disabled:cursor-not-allowed transition-colors"
          >
            {isSubmitting ? t.torrent_submitting : t.torrent_submit}
          </button>
        </form>

        {result && (
          <div
            className={`mt-3 px-3 py-2 rounded-md text-sm ${
              result.success
                ? "bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
                : "bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300"
            }`}
          >
            <div>{result.message}</div>
            {result.name && (
              <div className="mt-1 text-xs opacity-75">{result.name}</div>
            )}
          </div>
        )}

        <div className="mt-3 text-xs text-zinc-400 dark:text-zinc-600">
          Supports magnet links and .torrent URLs
        </div>
      </div>
    </section>
  );
}
