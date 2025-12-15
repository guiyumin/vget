import { useEffect } from "react";
import { FaCheck, FaXmark, FaCircleInfo } from "react-icons/fa6";
import clsx from "clsx";

export type ToastType = "success" | "error" | "info";

export interface ToastData {
  id: string;
  type: ToastType;
  message: string;
}

interface ToastProps {
  toast: ToastData;
  onDismiss: (id: string) => void;
}

export function Toast({ toast, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(toast.id);
    }, 3000);
    return () => clearTimeout(timer);
  }, [toast.id, onDismiss]);

  const icons = {
    success: <FaCheck className="text-green-500" />,
    error: <FaXmark className="text-red-500" />,
    info: <FaCircleInfo className="text-blue-500" />,
  };

  return (
    <div
      className={clsx(
        "flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg",
        "bg-white dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700",
        "animate-in slide-in-from-top-2 fade-in duration-200"
      )}
    >
      {icons[toast.type]}
      <span className="text-sm text-zinc-700 dark:text-zinc-300">
        {toast.message}
      </span>
      <button
        onClick={() => onDismiss(toast.id)}
        className="ml-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
      >
        <FaXmark className="text-xs" />
      </button>
    </div>
  );
}

interface ToastContainerProps {
  toasts: ToastData[];
  onDismiss: (id: string) => void;
}

export function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} onDismiss={onDismiss} />
      ))}
    </div>
  );
}
