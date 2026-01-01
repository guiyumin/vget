import { useReducer, useCallback, useRef, useEffect } from "react";
import {
  startAIProcessing,
  getAIJob,
  cancelAIJob,
  type AIJob,
  type ProcessingStep,
  type AIJobResult,
} from "../../utils/apis";

// State
interface ProcessingState {
  status: "idle" | "processing" | "completed" | "error";
  jobId: string | null;
  steps: ProcessingStep[];
  currentStepIndex: number;
  overallProgress: number;
  result: AIJobResult | null;
  error: string | null;
}

// Actions
type ProcessingAction =
  | { type: "START_PROCESSING"; jobId: string }
  | { type: "UPDATE_JOB"; job: AIJob }
  | { type: "PROCESSING_COMPLETE"; result: AIJobResult }
  | { type: "PROCESSING_ERROR"; error: string }
  | { type: "RESET" };

// Initial state
const initialState: ProcessingState = {
  status: "idle",
  jobId: null,
  steps: [],
  currentStepIndex: -1,
  overallProgress: 0,
  result: null,
  error: null,
};

// Reducer
function processingReducer(
  state: ProcessingState,
  action: ProcessingAction
): ProcessingState {
  switch (action.type) {
    case "START_PROCESSING":
      return {
        ...initialState,
        status: "processing",
        jobId: action.jobId,
      };

    case "UPDATE_JOB": {
      const job = action.job;
      const currentIdx = job.steps.findIndex(
        (s) => s.status === "in_progress"
      );

      return {
        ...state,
        steps: job.steps,
        currentStepIndex: currentIdx >= 0 ? currentIdx : state.currentStepIndex,
        overallProgress: job.overall_progress,
        error: job.error || null,
        status:
          job.status === "completed"
            ? "completed"
            : job.status === "failed" || job.status === "cancelled"
            ? "error"
            : "processing",
        result: job.result || null,
      };
    }

    case "PROCESSING_COMPLETE":
      return {
        ...state,
        status: "completed",
        result: action.result,
        overallProgress: 100,
      };

    case "PROCESSING_ERROR":
      return {
        ...state,
        status: "error",
        error: action.error,
      };

    case "RESET":
      return initialState;

    default:
      return state;
  }
}

// Hook
export function useProcessing() {
  const [state, dispatch] = useReducer(processingReducer, initialState);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const jobIdRef = useRef<string | null>(null);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, []);

  // Start polling for job status
  const startPolling = useCallback((jobId: string) => {
    jobIdRef.current = jobId;

    // Clear any existing polling
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
    }

    // Poll every 500ms
    pollingRef.current = setInterval(async () => {
      if (!jobIdRef.current) return;

      try {
        const res = await getAIJob(jobIdRef.current);
        if (res.code === 200 && res.data) {
          dispatch({ type: "UPDATE_JOB", job: res.data });

          // Stop polling if job is done
          if (
            res.data.status === "completed" ||
            res.data.status === "failed" ||
            res.data.status === "cancelled"
          ) {
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
          }
        }
      } catch (err) {
        console.error("Failed to poll job status:", err);
      }
    }, 500);
  }, []);

  // Start processing
  const startProcessing = useCallback(
    async (
      filePath: string,
      account: string,
      transcriptionModel: string,
      summarizationModel: string,
      includeSummary: boolean,
      pin?: string,
      useLocalASR?: boolean
    ) => {
      try {
        const res = await startAIProcessing({
          file_path: filePath,
          account: useLocalASR ? undefined : account,
          transcription_model: useLocalASR ? undefined : transcriptionModel,
          summarization_model: summarizationModel,
          pin,
          include_summary: includeSummary,
          use_local_asr: useLocalASR,
        });

        if (res.code === 200 && res.data?.job_id) {
          dispatch({ type: "START_PROCESSING", jobId: res.data.job_id });
          startPolling(res.data.job_id);
          return { success: true, jobId: res.data.job_id };
        } else {
          dispatch({ type: "PROCESSING_ERROR", error: res.message });
          return { success: false, error: res.message };
        }
      } catch (err) {
        const error = err instanceof Error ? err.message : "Unknown error";
        dispatch({ type: "PROCESSING_ERROR", error });
        return { success: false, error };
      }
    },
    [startPolling]
  );

  // Cancel current job
  const cancel = useCallback(async () => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }

    if (jobIdRef.current) {
      try {
        await cancelAIJob(jobIdRef.current);
      } catch (err) {
        console.error("Failed to cancel job:", err);
      }
    }

    dispatch({ type: "RESET" });
  }, []);

  // Reset state
  const reset = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
    jobIdRef.current = null;
    dispatch({ type: "RESET" });
  }, []);

  return {
    state,
    startProcessing,
    cancel,
    reset,
    isProcessing: state.status === "processing",
    isComplete: state.status === "completed",
    isError: state.status === "error",
  };
}
