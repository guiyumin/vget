import { useState } from "react";
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
import { Eye, EyeOff } from "lucide-react";
import type { Config } from "./types";

interface SiteSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

export function SiteSettings({ config, onUpdate }: SiteSettingsProps) {
  const [showTwitterToken, setShowTwitterToken] = useState(false);
  const [showBilibiliCookie, setShowBilibiliCookie] = useState(false);

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Twitter / X</CardTitle>
          <CardDescription>
            Required for downloading NSFW or protected content
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="twitter_auth_token">Auth Token</Label>
            <div className="flex gap-2">
              <Input
                id="twitter_auth_token"
                type={showTwitterToken ? "text" : "password"}
                value={config.twitter?.auth_token || ""}
                onChange={(e) =>
                  onUpdate({
                    twitter: {
                      ...config.twitter,
                      auth_token: e.target.value || null,
                    },
                  })
                }
                placeholder="Enter your auth_token cookie value"
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => setShowTwitterToken(!showTwitterToken)}
              >
                {showTwitterToken ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Find this in your browser's cookies after logging into Twitter/X
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Bilibili</CardTitle>
          <CardDescription>
            Required for high-quality downloads and member-only content
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="bilibili_cookie">Cookie</Label>
            <div className="flex gap-2">
              <Input
                id="bilibili_cookie"
                type={showBilibiliCookie ? "text" : "password"}
                value={config.bilibili?.cookie || ""}
                onChange={(e) =>
                  onUpdate({
                    bilibili: {
                      ...config.bilibili,
                      cookie: e.target.value || null,
                    },
                  })
                }
                placeholder="SESSDATA=...; bili_jct=...; DedeUserID=..."
                className="flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => setShowBilibiliCookie(!showBilibiliCookie)}
              >
                {showBilibiliCookie ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Copy full cookie string from your browser after logging in
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
