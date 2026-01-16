import { createFileRoute } from "@tanstack/react-router";
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

export const Route = createFileRoute("/media-tools")({
  component: MediaToolsPage,
});

interface Tool {
  id: string;
  title: string;
  description: string;
  icon: React.ReactNode;
  disabled?: boolean;
}

const tools: Tool[] = [
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
    id: "metadata",
    title: "Media Info",
    description: "View detailed information about media files",
    icon: <Info className="h-6 w-6" />,
  },
  {
    id: "audio-convert",
    title: "Audio Convert",
    description: "Convert between audio formats (MP3, AAC, FLAC, WAV)",
    icon: <FileType className="h-6 w-6" />,
  },
];

function MediaToolsPage() {
  const handleToolClick = (tool: Tool) => {
    if (tool.disabled) return;
    // TODO: Open tool-specific dialog or view
    console.log("Selected tool:", tool.id);
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
              className={`cursor-pointer transition-all hover:border-primary/50 hover:shadow-md ${
                tool.disabled ? "opacity-50 cursor-not-allowed" : ""
              }`}
              onClick={() => handleToolClick(tool)}
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
    </div>
  );
}
