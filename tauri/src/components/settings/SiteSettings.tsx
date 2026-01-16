import { useState, useEffect, useRef, useCallback } from "react";
import { QRCodeSVG } from "qrcode.react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Eye,
  EyeOff,
  CheckCircle2,
  Loader2,
  RefreshCw,
  LogOut,
  ExternalLink,
} from "lucide-react";
import { toast } from "sonner";
import type { Config } from "./types";
import {
  useAuthStore,
  generateBilibiliQR,
  pollBilibiliQR,
  saveBilibiliCookie,
  openXhsLoginWindow,
  QR_WAITING,
  QR_SCANNED,
  QR_EXPIRED,
  QR_CONFIRMED,
  type QRSession,
} from "@/stores/auth";

interface SiteSettingsProps {
  config: Config;
  onUpdate: (updates: Partial<Config>) => void;
}

interface CookieFields {
  sessdata: string;
  biliJct: string;
  dedeUserId: string;
}

function buildCookie(fields: CookieFields): string {
  const parts: string[] = [];
  if (fields.sessdata) parts.push(`SESSDATA=${fields.sessdata}`);
  if (fields.biliJct) parts.push(`bili_jct=${fields.biliJct}`);
  if (fields.dedeUserId) parts.push(`DedeUserID=${fields.dedeUserId}`);
  return parts.join("; ");
}

export function SiteSettings({ config, onUpdate }: SiteSettingsProps) {
  const [showTwitterToken, setShowTwitterToken] = useState(false);
  const { bilibili, xiaohongshu, setBilibiliStatus, logout, checkAuthStatus } =
    useAuthStore();

  useEffect(() => {
    checkAuthStatus();
  }, [checkAuthStatus]);

  return (
    <div className="space-y-6">
      {/* Twitter */}
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

      {/* Bilibili */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Bilibili
            {bilibili.status === "logged_in" && (
              <CheckCircle2 className="h-4 w-4 text-green-500" />
            )}
          </CardTitle>
          <CardDescription>
            {bilibili.status === "logged_in"
              ? `Logged in${bilibili.username ? ` as ${bilibili.username}` : ""}`
              : "Required for high-quality downloads and member-only content"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {bilibili.status === "logged_in" ? (
            <Button
              variant="outline"
              onClick={async () => {
                try {
                  await logout("bilibili");
                  toast.success("Logged out successfully");
                } catch {
                  toast.error("Failed to logout");
                }
              }}
            >
              <LogOut className="h-4 w-4 mr-2" />
              Logout
            </Button>
          ) : bilibili.status === "checking" ? (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Checking login status...
            </div>
          ) : (
            <Tabs defaultValue="qr">
              <TabsList className="grid w-full grid-cols-2">
                <TabsTrigger value="qr">QR Code</TabsTrigger>
                <TabsTrigger value="cookie">Cookie</TabsTrigger>
              </TabsList>
              <TabsContent value="qr" className="mt-4">
                <BilibiliQRLogin
                  onSuccess={(username) => {
                    setBilibiliStatus({ status: "logged_in", username });
                    toast.success(`Welcome, ${username || "User"}!`);
                  }}
                />
              </TabsContent>
              <TabsContent value="cookie" className="mt-4">
                <BilibiliCookieLogin
                  onSuccess={(username) => {
                    setBilibiliStatus({ status: "logged_in", username });
                    toast.success("Login successful!");
                  }}
                />
              </TabsContent>
            </Tabs>
          )}
        </CardContent>
      </Card>

      {/* Xiaohongshu */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Xiaohongshu
            {xiaohongshu.status === "logged_in" && (
              <CheckCircle2 className="h-4 w-4 text-green-500" />
            )}
          </CardTitle>
          <CardDescription>
            {xiaohongshu.status === "logged_in"
              ? xiaohongshu.username
                ? `Logged in as ${xiaohongshu.username}`
                : "Session cookies saved"
              : "Required for downloading videos and images"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {xiaohongshu.status === "logged_in" ? (
            <Button
              variant="outline"
              onClick={async () => {
                try {
                  await logout("xiaohongshu");
                  toast.success("Logged out successfully");
                } catch {
                  toast.error("Failed to logout");
                }
              }}
            >
              <LogOut className="h-4 w-4 mr-2" />
              Logout
            </Button>
          ) : xiaohongshu.status === "checking" ? (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Checking login status...
            </div>
          ) : (
            <XiaohongshuLogin />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function BilibiliQRLogin({
  onSuccess,
}: {
  onSuccess: (username?: string) => void;
}) {
  const [qrSession, setQrSession] = useState<QRSession | null>(null);
  const [qrStatus, setQrStatus] = useState<number | null>(null);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollIntervalRef = useRef<number | null>(null);

  const generateQR = useCallback(async () => {
    setGenerating(true);
    setError(null);
    setQrStatus(null);

    try {
      const session = await generateBilibiliQR();
      setQrSession(session);
      setQrStatus(QR_WAITING);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to generate QR code"
      );
    } finally {
      setGenerating(false);
    }
  }, []);

  const pollStatus = useCallback(async () => {
    if (!qrSession) return;

    try {
      const result = await pollBilibiliQR(qrSession.qrcode_key);
      setQrStatus(result.status);

      if (result.status === QR_CONFIRMED) {
        if (pollIntervalRef.current) {
          clearInterval(pollIntervalRef.current);
          pollIntervalRef.current = null;
        }
        onSuccess(result.username);
      } else if (result.status === QR_EXPIRED) {
        if (pollIntervalRef.current) {
          clearInterval(pollIntervalRef.current);
          pollIntervalRef.current = null;
        }
      }
    } catch (err) {
      console.error("Poll error:", err);
    }
  }, [qrSession, onSuccess]);

  useEffect(() => {
    generateQR();
  }, [generateQR]);

  useEffect(() => {
    const shouldPoll =
      qrSession && (qrStatus === QR_WAITING || qrStatus === QR_SCANNED);

    if (shouldPoll) {
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
      pollIntervalRef.current = window.setInterval(pollStatus, 1500);
    }

    return () => {
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
        pollIntervalRef.current = null;
      }
    };
  }, [qrSession, qrStatus, pollStatus]);

  const getStatusText = () => {
    switch (qrStatus) {
      case QR_WAITING:
        return "Scan with Bilibili app";
      case QR_SCANNED:
        return "Confirm login on your phone";
      case QR_EXPIRED:
        return "QR code expired";
      case QR_CONFIRMED:
        return "Login successful!";
      default:
        return "";
    }
  };

  return (
    <div className="flex flex-col items-center">
      <div className="mb-4 p-4 bg-white rounded-lg">
        {generating ? (
          <div className="w-40 h-40 flex items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <div className="w-40 h-40 flex items-center justify-center text-destructive text-center text-sm p-4">
            {error}
          </div>
        ) : qrSession ? (
          <QRCodeSVG
            value={qrSession.url}
            size={160}
            level="L"
            className={qrStatus === QR_EXPIRED ? "opacity-30" : ""}
          />
        ) : (
          <div className="w-40 h-40 flex items-center justify-center text-muted-foreground">
            Waiting...
          </div>
        )}
      </div>

      <div className="mb-4 text-center text-sm">
        {qrStatus === QR_SCANNED ? (
          <span className="text-green-600 font-medium flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            {getStatusText()}
          </span>
        ) : qrStatus === QR_EXPIRED ? (
          <span className="text-destructive">{getStatusText()}</span>
        ) : (
          <span className="text-muted-foreground">{getStatusText()}</span>
        )}
      </div>

      {(qrStatus === QR_EXPIRED || error) && (
        <Button onClick={generateQR} disabled={generating} variant="outline" size="sm">
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh QR Code
        </Button>
      )}
    </div>
  );
}

function BilibiliCookieLogin({
  onSuccess,
}: {
  onSuccess: (username?: string) => void;
}) {
  const [fields, setFields] = useState<CookieFields>({
    sessdata: "",
    biliJct: "",
    dedeUserId: "",
  });
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    const cookie = buildCookie(fields);
    if (!cookie) {
      toast.error("Please fill in at least one field");
      return;
    }

    setSaving(true);
    try {
      await saveBilibiliCookie(cookie);
      onSuccess();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save cookie");
    } finally {
      setSaving(false);
    }
  };

  const hasAnyInput = fields.sessdata || fields.biliJct || fields.dedeUserId;

  return (
    <div className="space-y-4">
      <div className="p-3 bg-muted rounded-lg text-sm text-muted-foreground">
        <p className="font-medium mb-2">How to get cookies:</p>
        <ol className="list-decimal list-inside space-y-1">
          <li>Open bilibili.com and login</li>
          <li>Press F12 to open DevTools</li>
          <li>Go to Application tab → Cookies</li>
          <li>Copy the values below</li>
        </ol>
      </div>

      <div className="space-y-3">
        <div className="space-y-2">
          <Label htmlFor="sessdata">SESSDATA</Label>
          <Input
            id="sessdata"
            value={fields.sessdata}
            onChange={(e) =>
              setFields((f) => ({ ...f, sessdata: e.target.value }))
            }
            placeholder="Paste SESSDATA value"
            className="font-mono text-sm"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="bili_jct">bili_jct</Label>
          <Input
            id="bili_jct"
            value={fields.biliJct}
            onChange={(e) =>
              setFields((f) => ({ ...f, biliJct: e.target.value }))
            }
            placeholder="Paste bili_jct value"
            className="font-mono text-sm"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dedeUserId">DedeUserID</Label>
          <Input
            id="dedeUserId"
            value={fields.dedeUserId}
            onChange={(e) =>
              setFields((f) => ({ ...f, dedeUserId: e.target.value }))
            }
            placeholder="Paste DedeUserID value"
            className="font-mono text-sm"
          />
        </div>
      </div>

      <Button onClick={handleSave} disabled={saving || !hasAnyInput} className="w-full">
        {saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
        Save
      </Button>
    </div>
  );
}

function XiaohongshuLogin() {
  const { checkAuthStatus } = useAuthStore();
  const [opening, setOpening] = useState(false);

  const handleOpenLogin = async () => {
    setOpening(true);
    try {
      await openXhsLoginWindow();
      setTimeout(async () => {
        await checkAuthStatus();
        const state = useAuthStore.getState();
        if (state.xiaohongshu.status === "logged_in") {
          toast.success("Login successful!");
        }
      }, 1000);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to open login window"
      );
    } finally {
      setOpening(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="p-3 bg-muted rounded-lg text-sm text-muted-foreground">
        <p className="mb-2">Click below to open a login window:</p>
        <ul className="list-disc list-inside space-y-1">
          <li>Scan QR with Xiaohongshu app</li>
          <li>Or login with phone number</li>
          <li>Close the window when done</li>
        </ul>
      </div>

      <div className="flex gap-2">
        <Button onClick={handleOpenLogin} disabled={opening} className="flex-1">
          {opening ? (
            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
          ) : (
            <ExternalLink className="h-4 w-4 mr-2" />
          )}
          {opening ? "Opening..." : "Open Login Window"}
        </Button>

        <Button variant="outline" onClick={checkAuthStatus}>
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
