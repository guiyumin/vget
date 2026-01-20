import { Button } from "./button";
import { Upload, FileText, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useDropZone } from "@/hooks/useDropZone";
import { toast } from "sonner";

interface FileDropInputProps {
  /** Current file path value */
  value: string;
  /** Placeholder text when no file is selected */
  placeholder?: string;
  /** File extensions to accept (without dot), e.g. ["mp4", "mkv"] */
  accept?: string[];
  /** Callback when file is selected via dialog */
  onSelectClick: () => void;
  /** Callback when file is dropped */
  onDrop: (path: string) => void;
  /** Callback to clear the file */
  onClear?: () => void;
  /** Whether the input is disabled */
  disabled?: boolean;
  /** Custom class name for the container */
  className?: string;
  /** Error message for invalid file drop */
  invalidDropMessage?: string;
  /** Hint text showing accepted file types */
  acceptHint?: string;
}

export function FileDropInput({
  value,
  placeholder = "Drop a file here or click to select",
  accept,
  onSelectClick,
  onDrop,
  onClear,
  disabled = false,
  className,
  invalidDropMessage = "Invalid file type",
  acceptHint,
}: FileDropInputProps) {
  const { ref, isDragging } = useDropZone<HTMLDivElement>({
    accept,
    onDrop: (paths) => {
      if (paths.length > 0) {
        onDrop(paths[0]);
      }
    },
    onInvalidDrop: () => {
      toast.error(invalidDropMessage);
    },
    enabled: !disabled && !value,
  });

  const getFileName = (path: string) => path.split(/[/\\]/).pop() || path;

  // Show selected file state
  if (value) {
    return (
      <div
        className={cn(
          "flex items-center gap-3 p-3 bg-muted rounded-lg border border-border",
          className
        )}
      >
        <FileText className="h-5 w-5 shrink-0 text-muted-foreground" />
        <span className="flex-1 text-sm truncate" title={value}>
          {getFileName(value)}
        </span>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={onSelectClick}
            disabled={disabled}
          >
            Change
          </Button>
          {onClear && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onClear}
              disabled={disabled}
              className="px-2"
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>
    );
  }

  // Show drop zone state
  return (
    <div
      ref={ref}
      onClick={disabled ? undefined : onSelectClick}
      className={cn(
        "border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors",
        isDragging && !disabled
          ? "border-primary bg-primary/5"
          : "border-muted-foreground/25 hover:border-muted-foreground/50 hover:bg-muted/50",
        disabled && "opacity-50 cursor-not-allowed",
        className
      )}
    >
      <Upload className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">
        {isDragging ? "Drop file here..." : placeholder}
      </p>
      {acceptHint && (
        <p className="text-xs text-muted-foreground/70 mt-1">
          {acceptHint}
        </p>
      )}
    </div>
  );
}
