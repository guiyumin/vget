import {
  FaCircle,
  FaSpinner,
  FaCheck,
  FaMinus,
  FaXmark,
} from "react-icons/fa6";
import type { ProcessingStep, StepStatus } from "../../utils/apis";
import type { StepKey } from "./types";
import type { UITranslations } from "../../utils/translations";
import clsx from "clsx";

interface ProcessingStepperProps {
  steps: ProcessingStep[];
  currentStepIndex: number;
  overallProgress: number;
  isProcessing: boolean;
  emptyText?: string;
  translations: UITranslations;
}

// Map step keys to translation keys
const stepNameMap: Record<StepKey, keyof UITranslations> = {
  extract_audio: "ai_step_extract_audio",
  compress_audio: "ai_step_compress_audio",
  chunk_audio: "ai_step_chunk_audio",
  transcribe: "ai_step_transcribe",
  merge: "ai_step_merge",
  summarize: "ai_step_summarize",
};


function getStepIcon(status: StepStatus) {
  switch (status) {
    case "completed":
      return <FaCheck className="text-green-500" />;
    case "in_progress":
      return <FaSpinner className="animate-spin text-blue-500" />;
    case "skipped":
      return <FaMinus className="text-zinc-400" />;
    case "failed":
      return <FaXmark className="text-red-500" />;
    case "pending":
    default:
      return <FaCircle className="text-zinc-300 dark:text-zinc-600 text-xs" />;
  }
}

function getStepTextColor(status: StepStatus): string {
  switch (status) {
    case "completed":
      return "text-green-600 dark:text-green-400";
    case "in_progress":
      return "text-blue-600 dark:text-blue-400 font-medium";
    case "skipped":
      return "text-zinc-400 dark:text-zinc-500";
    case "failed":
      return "text-red-600 dark:text-red-400";
    case "pending":
    default:
      return "text-zinc-500 dark:text-zinc-400";
  }
}

// Helper to translate step name
function getStepName(step: ProcessingStep, t: UITranslations): string {
  const key = step.key as StepKey;
  const translationKey = stepNameMap[key];
  return translationKey ? (t[translationKey] as string) : step.name;
}


export function ProcessingStepper({
  steps,
  currentStepIndex,
  overallProgress,
  isProcessing,
  emptyText = "Select a file to start",
  translations: t,
}: ProcessingStepperProps) {
  if (steps.length === 0) {
    return (
      <div className="text-sm text-zinc-500 dark:text-zinc-400 text-center py-4">
        {emptyText}
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {/* Overall progress bar */}
      {isProcessing && (
        <div className="mb-3">
          <div className="flex justify-between text-xs text-zinc-500 dark:text-zinc-400 mb-1">
            <span>Progress</span>
            <span>{Math.round(overallProgress)}%</span>
          </div>
          <div className="h-1.5 bg-zinc-200 dark:bg-zinc-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-blue-500 transition-all duration-300"
              style={{ width: `${overallProgress}%` }}
            />
          </div>
        </div>
      )}

      {/* Steps list */}
      <div className="space-y-0.5">
        {steps.map((step, index) => (
          <div
            key={step.key}
            className={clsx(
              "flex items-center gap-2 px-2 py-1.5 rounded-md transition-colors",
              index === currentStepIndex &&
                step.status === "in_progress" &&
                "bg-blue-50 dark:bg-blue-900/20 border-l-2 border-blue-500",
              step.status === "failed" && "bg-red-50 dark:bg-red-900/20"
            )}
          >
            <div className="w-4 h-4 flex items-center justify-center shrink-0">
              {getStepIcon(step.status)}
            </div>
            <div className="flex-1 min-w-0">
              <div
                className={clsx(
                  "text-sm truncate",
                  getStepTextColor(step.status)
                )}
              >
                {getStepName(step, t)}
              </div>
              {step.detail && step.status !== "pending" && (
                <div className="text-xs text-zinc-400 dark:text-zinc-500 truncate">
                  {step.detail}
                </div>
              )}
            </div>
            {step.status === "in_progress" &&
              step.progress > 0 &&
              step.progress < 100 && (
                <div className="text-xs text-blue-500 shrink-0">
                  {Math.round(step.progress)}%
                </div>
              )}
          </div>
        ))}
      </div>
    </div>
  );
}
