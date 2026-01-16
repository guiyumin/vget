import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { open } from "@tauri-apps/plugin-dialog";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  FileVideo,
  FileAudio,
  Scissors,
  Minimize2,
  Image,
  FileType,
  Info,
} from "lucide-react";
import { toast } from "sonner";
import { MediaInfo, ToolId, Config } from "./types";
import { MediaInfoDialog } from "./dialogs/MediaInfoDialog";
import { ConvertDialog } from "./dialogs/ConvertDialog";
import { CompressDialog } from "./dialogs/CompressDialog";
import { TrimDialog } from "./dialogs/TrimDialog";
import { ExtractAudioDialog } from "./dialogs/ExtractAudioDialog";
import { ExtractFramesDialog } from "./dialogs/ExtractFramesDialog";
import { AudioConvertDialog } from "./dialogs/AudioConvertDialog";

interface Tool {
  id: ToolId;
  title: string;
  description: string;
  icon: React.ReactNode;
}

const tools: Tool[] = [
  {
    id: "info",
    title: "Media Info",
    description: "View detailed information about media files",
    icon: <Info className="h-6 w-6" />,
  },
  {
    id: "convert",
    title: "Convert",
    description: "Convert between video formats (MP4, WebM, MKV, MOV)",
    icon: <FileVideo className="h-6 w-6" />,
  },
  {
    id: "compress",
    title: "Compress",
    description: "Reduce file size while maintaining quality",
    icon: <Minimize2 className="h-6 w-6" />,
  },
  {
    id: "trim",
    title: "Trim",
    description: "Cut clips from videos with start and end times",
    icon: <Scissors className="h-6 w-6" />,
  },
  {
    id: "extract-audio",
    title: "Extract Audio",
    description: "Extract audio track from video files",
    icon: <FileAudio className="h-6 w-6" />,
  },
  {
    id: "extract-frames",
    title: "Extract Frames",
    description: "Extract images or thumbnails from video",
    icon: <Image className="h-6 w-6" />,
  },
  {
    id: "audio-convert",
    title: "Audio Convert",
    description: "Convert between audio formats (MP3, AAC, FLAC, WAV)",
    icon: <FileType className="h-6 w-6" />,
  },
];

export function MediaToolsPage() {
  const [activeTool, setActiveTool] = useState<ToolId | null>(null);
  const [inputFile, setInputFile] = useState("");
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [mediaInfo, setMediaInfo] = useState<MediaInfo | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [config, setConfig] = useState<Config | null>(null);

  useEffect(() => {
    invoke<Config>("get_config")
      .then(setConfig)
      .catch(console.error);
  }, []);

  useEffect(() => {
    const unlistenProgress = listen<{ jobId: string; progress: number }>(
      "ffmpeg-progress",
      (event) => {
        if (event.payload.jobId === jobId && mediaInfo?.duration) {
          const percent = Math.min(
            100,
            (event.payload.progress / mediaInfo.duration) * 100
          );
          setProgress(percent);
        }
      }
    );

    const unlistenComplete = listen<{ jobId: string; outputPath: string }>(
      "ffmpeg-complete",
      (event) => {
        if (event.payload.jobId === jobId) {
          setLoading(false);
          setProgress(100);
          toast.success("Operation completed successfully!");
          setTimeout(() => {
            setActiveTool(null);
            resetState();
          }, 1500);
        }
      }
    );

    const unlistenError = listen<{ jobId: string; error: string }>(
      "ffmpeg-error",
      (event) => {
        if (event.payload.jobId === jobId) {
          setLoading(false);
          toast.error(event.payload.error);
        }
      }
    );

    return () => {
      unlistenProgress.then((fn) => fn());
      unlistenComplete.then((fn) => fn());
      unlistenError.then((fn) => fn());
    };
  }, [jobId, mediaInfo]);

  const resetState = () => {
    setInputFile("");
    setMediaInfo(null);
    setProgress(0);
    setJobId(null);
  };

  const selectInputFile = async () => {
    const file = await open({
      multiple: false,
      filters: [
        { name: "Media", extensions: ["mp4", "mkv", "webm", "mov", "avi", "mp3", "aac", "flac", "wav", "ogg"] },
      ],
    });
    if (file) {
      setInputFile(file);
      try {
        const info = await invoke<MediaInfo>("ffmpeg_get_media_info", {
          inputPath: file,
        });
        setMediaInfo(info);
      } catch (e) {
        console.error("Failed to get media info:", e);
      }
    }
  };

  const closeDialog = () => {
    if (!loading) {
      setActiveTool(null);
      resetState();
    }
  };

  const dialogProps = {
    inputFile,
    outputDir: config?.output_dir || "",
    loading,
    progress,
    mediaInfo,
    onSelectInput: selectInputFile,
    onClose: closeDialog,
    setLoading,
    setProgress,
    setJobId,
  };

  return (
    <div className="h-full">
      <header className="h-14 border-b border-border flex items-center px-6">
        <h1 className="text-xl font-semibold">Media Tools</h1>
      </header>

      <div className="p-6">
        <p className="text-sm text-muted-foreground mb-6">
          Process your media files with FFmpeg-powered tools
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {tools.map((tool) => (
            <Card
              key={tool.id}
              className="cursor-pointer transition-all hover:border-primary/50 hover:shadow-md"
              onClick={() => setActiveTool(tool.id)}
            >
              <CardHeader className="flex flex-row items-start gap-4 space-y-0">
                <div className="p-2 rounded-lg bg-primary/10 text-primary">
                  {tool.icon}
                </div>
                <div className="flex-1">
                  <CardTitle className="text-base">{tool.title}</CardTitle>
                  <CardDescription className="text-sm mt-1">
                    {tool.description}
                  </CardDescription>
                </div>
              </CardHeader>
            </Card>
          ))}
        </div>
      </div>

      <MediaInfoDialog open={activeTool === "info"} {...dialogProps} />
      <ConvertDialog open={activeTool === "convert"} {...dialogProps} />
      <CompressDialog open={activeTool === "compress"} {...dialogProps} />
      <TrimDialog open={activeTool === "trim"} {...dialogProps} />
      <ExtractAudioDialog open={activeTool === "extract-audio"} {...dialogProps} />
      <ExtractFramesDialog open={activeTool === "extract-frames"} {...dialogProps} />
      <AudioConvertDialog open={activeTool === "audio-convert"} {...dialogProps} />
    </div>
  );
}
