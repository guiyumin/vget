import { useEffect, useRef, useState, RefObject } from "react";
import { listen } from "@tauri-apps/api/event";

interface DragDropPayload {
  paths: string[];
  position: { x: number; y: number };
}

interface UseDropZoneOptions<T extends HTMLElement> {
  /**
   * Optional ref to use - if not provided, hook creates its own
   */
  ref?: RefObject<T | null>;
  /**
   * File extensions to accept (without dot), e.g. ["txt", "md"]
   */
  accept?: string[];
  /**
   * Callback when valid file(s) are dropped
   */
  onDrop: (paths: string[]) => void;
  /**
   * Callback when invalid file is dropped (wrong extension)
   */
  onInvalidDrop?: (paths: string[], extension: string | undefined) => void;
  /**
   * Whether the drop zone is enabled
   */
  enabled?: boolean;
}

/**
 * Check if a point is within an element's bounding box
 */
function isPointInElement(x: number, y: number, element: HTMLElement): boolean {
  const rect = element.getBoundingClientRect();
  return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
}

/**
 * Hook for creating a drop zone that responds to Tauri drag-drop events.
 * Uses drop position to check if drop occurred within the element's bounds.
 */
export function useDropZone<T extends HTMLElement = HTMLDivElement>(
  options: UseDropZoneOptions<T>
) {
  const { ref: externalRef, accept, onDrop, onInvalidDrop, enabled = true } = options;
  const internalRef = useRef<T>(null);
  const ref = externalRef || internalRef;
  const [isDragging, setIsDragging] = useState(false);

  useEffect(() => {
    if (!enabled) return;

    let unlistenDrop: (() => void) | undefined;
    let unlistenEnter: (() => void) | undefined;
    let unlistenLeave: (() => void) | undefined;

    const setupListeners = async () => {
      // Listen for file drop - check if position is within this element
      unlistenDrop = await listen<DragDropPayload>(
        "tauri://drag-drop",
        (event) => {
          setIsDragging(false);

          const element = ref.current;
          if (!element) return;

          const { paths, position } = event.payload;
          if (!paths || paths.length === 0) return;

          // Check if drop position is within this element's bounds
          if (!isPointInElement(position.x, position.y, element)) {
            return; // Drop was outside this element
          }

          // Check file extension if accept list is provided
          if (accept && accept.length > 0) {
            const filePath = paths[0];
            const ext = filePath.toLowerCase().split(".").pop();

            if (!ext || !accept.includes(ext)) {
              onInvalidDrop?.(paths, ext);
              return;
            }
          }

          onDrop(paths);
        }
      );

      // Track when dragging enters the window
      unlistenEnter = await listen("tauri://drag-enter", () => {
        setIsDragging(true);
      });

      // Track when dragging leaves the window
      unlistenLeave = await listen("tauri://drag-leave", () => {
        setIsDragging(false);
      });
    };

    setupListeners();

    return () => {
      unlistenDrop?.();
      unlistenEnter?.();
      unlistenLeave?.();
    };
  }, [enabled, accept, onDrop, onInvalidDrop, ref]);

  return {
    ref,
    isDragging, // true when files are being dragged over the window
  };
}
