import { useRef } from "react";
import {
  FaMusic,
  FaVideo,
  FaUpload,
  FaSpinner,
  FaCheck,
} from "react-icons/fa6";
import clsx from "clsx";
import type { AudioFile } from "../../utils/apis";

interface FileSelectorProps {
  files: AudioFile[];
  selectedPath: string | null;
  onSelect: (file: AudioFile) => void;
  onUpload: (file: File) => void;
  uploading: boolean;
  disabled: boolean;
}

const VIDEO_EXTENSIONS = [
  ".mp4",
  ".webm",
  ".mkv",
  ".avi",
  ".mov",
  ".flv",
  ".wmv",
];

function isVideoFile(filename: string): boolean {
  const ext = filename.toLowerCase().slice(filename.lastIndexOf("."));
  return VIDEO_EXTENSIONS.includes(ext);
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function FileSelector({
  files,
  selectedPath,
  onSelect,
  onUpload,
  uploading,
  disabled,
}: FileSelectorProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      onUpload(file);
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 overflow-hidden flex flex-col h-full">
      {/* Header */}
      <div className="px-3 py-2 border-b border-zinc-200 dark:border-zinc-700 flex items-center justify-between">
        <h3 className="text-sm font-medium text-zinc-900 dark:text-white">
          Media Files
        </h3>
        <div>
          <input
            ref={fileInputRef}
            type="file"
            accept=".mp3,.m4a,.wav,.aac,.ogg,.flac,.opus,.wma,.mp4,.webm,.mkv,.avi,.mov,.flv,.wmv"
            className="hidden"
            onChange={handleFileChange}
            disabled={disabled || uploading}
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={disabled || uploading}
            className={clsx(
              "flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md transition-colors",
              disabled || uploading
                ? "bg-zinc-100 dark:bg-zinc-700 text-zinc-400 cursor-not-allowed"
                : "bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50"
            )}
          >
            {uploading ? (
              <>
                <FaSpinner className="animate-spin" />
                <span>Uploading...</span>
              </>
            ) : (
              <>
                <FaUpload />
                <span>Upload</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* File list */}
      <div className="flex-1 min-h-0 overflow-y-auto">
        {files.length === 0 ? (
          <div className="px-3 py-6 text-center text-sm text-zinc-400 dark:text-zinc-500">
            No media files found
          </div>
        ) : (
          <div className="divide-y divide-zinc-100 dark:divide-zinc-700/50">
            {files.map((file) => {
              const isSelected = file.full_path === selectedPath;
              const isVideo = isVideoFile(file.name);

              return (
                <button
                  key={file.full_path}
                  onClick={() => onSelect(file)}
                  disabled={disabled}
                  className={clsx(
                    "w-full flex items-center gap-2 px-3 py-2 text-left transition-colors",
                    isSelected
                      ? "bg-blue-50 dark:bg-blue-900/20"
                      : "hover:bg-zinc-50 dark:hover:bg-zinc-700/50",
                    disabled && "opacity-50 cursor-not-allowed"
                  )}
                >
                  {/* Icon */}
                  <div
                    className={clsx(
                      "w-5 h-5 flex items-center justify-center shrink-0",
                      isVideo ? "text-purple-500" : "text-blue-500"
                    )}
                  >
                    {isVideo ? <FaVideo /> : <FaMusic />}
                  </div>

                  {/* Filename */}
                  <div className="flex-1 min-w-0">
                    <div
                      className={clsx(
                        "text-sm truncate",
                        isSelected
                          ? "text-blue-700 dark:text-blue-300 font-medium"
                          : "text-zinc-700 dark:text-zinc-300"
                      )}
                    >
                      {file.name}
                    </div>
                  </div>

                  {/* Size */}
                  <div className="text-xs text-zinc-400 dark:text-zinc-500 shrink-0">
                    {formatFileSize(file.size)}
                  </div>

                  {/* Status badges */}
                  <div className="flex gap-1 shrink-0">
                    {file.has_transcript && (
                      <span className="w-4 h-4 flex items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400">
                        <FaCheck className="text-[8px]" />
                      </span>
                    )}
                    {file.has_summary && (
                      <span className="w-4 h-4 flex items-center justify-center rounded-full bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400">
                        <FaCheck className="text-[8px]" />
                      </span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
