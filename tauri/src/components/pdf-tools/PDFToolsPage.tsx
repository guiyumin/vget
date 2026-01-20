import { useState, useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { Combine, Image, Trash2, Droplets, FileText } from "lucide-react";
import { PdfToolId, Config } from "./types";
import { MergePdfPanel, ImagesToPdfPanel, DeletePagesPanel, RemoveWatermarkPanel, Md2PdfPanel } from "./panels";

interface Tool {
  id: PdfToolId;
  titleKey: string;
  descKey: string;
  icon: React.ReactNode;
}

const toolsConfig: Tool[] = [
  {
    id: "merge",
    titleKey: "pdfTools.tools.merge.title",
    descKey: "pdfTools.tools.merge.desc",
    icon: <Combine className="h-4 w-4" />,
  },
  {
    id: "images-to-pdf",
    titleKey: "pdfTools.tools.imagesToPdf.title",
    descKey: "pdfTools.tools.imagesToPdf.desc",
    icon: <Image className="h-4 w-4" />,
  },
  {
    id: "delete-pages",
    titleKey: "pdfTools.tools.deletePages.title",
    descKey: "pdfTools.tools.deletePages.desc",
    icon: <Trash2 className="h-4 w-4" />,
  },
  {
    id: "remove-watermark",
    titleKey: "pdfTools.tools.removeWatermark.title",
    descKey: "pdfTools.tools.removeWatermark.desc",
    icon: <Droplets className="h-4 w-4" />,
  },
  {
    id: "md-to-pdf",
    titleKey: "pdfTools.tools.md2pdf.title",
    descKey: "pdfTools.tools.md2pdf.desc",
    icon: <FileText className="h-4 w-4" />,
  },
];

export function PDFToolsPage() {
  const { t } = useTranslation();
  const [activeTool, setActiveTool] = useState<PdfToolId>("merge");
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<Config | null>(null);

  useEffect(() => {
    invoke<Config>("get_config")
      .then(setConfig)
      .catch(console.error);
  }, []);

  const handleToolChange = (toolId: PdfToolId) => {
    if (!loading) {
      setActiveTool(toolId);
    }
  };

  const panelProps = {
    outputDir: config?.output_dir || "",
    loading,
    setLoading,
  };

  const activeToolData = toolsConfig.find((tool) => tool.id === activeTool);

  const renderPanel = () => {
    switch (activeTool) {
      case "merge":
        return <MergePdfPanel {...panelProps} />;
      case "images-to-pdf":
        return <ImagesToPdfPanel {...panelProps} />;
      case "delete-pages":
        return <DeletePagesPanel {...panelProps} />;
      case "remove-watermark":
        return <RemoveWatermarkPanel {...panelProps} />;
      case "md-to-pdf":
        return <Md2PdfPanel {...panelProps} />;
      default:
        return null;
    }
  };

  return (
    <div className="h-full flex flex-col">
      <header className="h-14 border-b border-border flex items-center px-6 shrink-0">
        <h1 className="text-xl font-semibold">{t("pdfTools.title")}</h1>
      </header>

      <div className="flex-1 flex min-h-0">
        {/* Left pane - Tool list */}
        <div className="w-56 border-r border-border p-2 overflow-y-auto shrink-0">
          <div className="space-y-1">
            {toolsConfig.map((tool) => (
              <button
                key={tool.id}
                onClick={() => handleToolChange(tool.id)}
                disabled={loading}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2 rounded-md text-left transition-colors",
                  "hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed",
                  activeTool === tool.id
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <span
                  className={cn(
                    "shrink-0",
                    activeTool === tool.id ? "text-primary" : ""
                  )}
                >
                  {tool.icon}
                </span>
                <span className="text-sm font-medium truncate">{t(tool.titleKey)}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Right pane - Tool content */}
        <div className="flex-1 p-6 overflow-y-auto">
          {activeToolData && (
            <div className="max-w-lg">
              <div className="mb-6">
                <h2 className="text-lg font-semibold">{t(activeToolData.titleKey)}</h2>
                <p className="text-sm text-muted-foreground mt-1">
                  {t(activeToolData.descKey)}
                </p>
              </div>
              {renderPanel()}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
