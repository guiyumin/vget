import { useEffect } from "react";
import { open } from "@tauri-apps/plugin-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Folder } from "lucide-react";
import type { Config } from "./types";

// Use global theme function from main.tsx
const applyTheme = (window as any).__applyTheme as (theme: string) => void;

interface GeneralSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function GeneralSettings({ config, onUpdate }: GeneralSettingsProps) {
  const theme = config.theme || "light";

  useEffect(() => {
    applyTheme?.(theme);
  }, [theme]);

  const handleSelectFolder = async () => {
    const selected = await open({
      directory: true,
      multiple: false,
      title: "Select Download Directory",
    });
    if (selected) {
      onUpdate({ output_dir: selected as string });
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Downloads</CardTitle>
          <CardDescription>Configure download preferences</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="output_dir">Download Location</Label>
            <div className="flex gap-2">
              <Input
                id="output_dir"
                value={config.output_dir}
                onChange={(e) => onUpdate({ output_dir: e.target.value })}
                className="flex-1"
              />
              <Button variant="outline" size="icon" onClick={handleSelectFolder}>
                <Folder className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Where downloaded files will be saved
            </p>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="format">Default Format</Label>
            <Select
              value={config.format}
              onValueChange={(value) => onUpdate({ format: value })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select format" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="mp4">MP4</SelectItem>
                <SelectItem value="webm">WebM</SelectItem>
                <SelectItem value="best">Best Available</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="quality">Default Quality</Label>
            <Select
              value={config.quality}
              onValueChange={(value) => onUpdate({ quality: value })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select quality" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="best">Best Available</SelectItem>
                <SelectItem value="1080p">1080p</SelectItem>
                <SelectItem value="720p">720p</SelectItem>
                <SelectItem value="480p">480p</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-sm text-muted-foreground">
              Preferred video quality when multiple options are available
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Language</CardTitle>
          <CardDescription>Application display language</CardDescription>
        </CardHeader>
        <CardContent>
          <Select
            value={config.language}
            onValueChange={(value) => onUpdate({ language: value })}
          >
            <SelectTrigger>
              <SelectValue placeholder="Select language" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="en">English</SelectItem>
              <SelectItem value="zh">中文</SelectItem>
              <SelectItem value="jp">日本語</SelectItem>
              <SelectItem value="kr">한국어</SelectItem>
              <SelectItem value="es">Español</SelectItem>
              <SelectItem value="fr">Français</SelectItem>
              <SelectItem value="de">Deutsch</SelectItem>
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Theme</CardTitle>
          <CardDescription>Choose your preferred appearance</CardDescription>
        </CardHeader>
        <CardContent>
          <Select value={theme} onValueChange={(value) => onUpdate({ theme: value })}>
            <SelectTrigger>
              <SelectValue placeholder="Select theme" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="light">Light</SelectItem>
              <SelectItem value="dark">Dark</SelectItem>
              <SelectItem value="system">System</SelectItem>
            </SelectContent>
          </Select>
        </CardContent>
      </Card>
    </div>
  );
}
