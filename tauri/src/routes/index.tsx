import { createFileRoute, Link as RouterLink } from "@tanstack/react-router";
import { useState } from "react";
import { Download, Settings, Folder, Link } from "lucide-react";
import logo from "@/assets/logo.png";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const [url, setUrl] = useState("");
  const [isExtracting, setIsExtracting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) return;

    setIsExtracting(true);
    // TODO: Call Tauri extract_media command
    console.log("Extracting:", url);
    setIsExtracting(false);
  };

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b border-border">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <img src={logo} alt="vget" className="h-8 w-8" />
            <h1 className="text-xl font-semibold">VGet</h1>
          </div>
          <RouterLink
            to="/settings"
            className="p-2 rounded-lg hover:bg-muted transition-colors"
          >
            <Settings className="h-5 w-5 text-muted-foreground" />
          </RouterLink>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        {/* URL Input */}
        <form onSubmit={handleSubmit} className="max-w-2xl mx-auto">
          <div className="relative">
            <Link className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground" />
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="Paste video URL here..."
              className="w-full pl-12 pr-24 py-4 rounded-xl border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <button
              type="submit"
              disabled={isExtracting || !url.trim()}
              className="absolute right-2 top-1/2 -translate-y-1/2 px-4 py-2 rounded-lg bg-primary text-primary-foreground font-medium disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
            >
              {isExtracting ? "..." : "Download"}
            </button>
          </div>
        </form>

        {/* Supported Sites */}
        <div className="mt-8 max-w-2xl mx-auto">
          <p className="text-sm text-muted-foreground text-center">
            Supports Twitter/X, Bilibili, Xiaohongshu, YouTube, Apple Podcasts,
            and direct URLs
          </p>
        </div>

        {/* Downloads Section */}
        <div className="mt-12 max-w-2xl mx-auto">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-medium">Downloads</h2>
            <button className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors">
              <Folder className="h-4 w-4" />
              Open folder
            </button>
          </div>

          {/* Empty State */}
          <div className="border border-dashed border-border rounded-xl p-12 text-center">
            <Download className="h-12 w-12 text-muted-foreground/50 mx-auto mb-4" />
            <p className="text-muted-foreground">
              No downloads yet. Paste a URL above to get started.
            </p>
          </div>
        </div>
      </main>
    </div>
  );
}
