import { useState, useEffect, useRef, useCallback } from "react";
import { useApp } from "../context/AppContext";
import { setConfigValue } from "../utils/apis";
import { QRCodeSVG } from "qrcode.react";

type LoginMethod = "qr" | "cookie";

// QR Status codes from Bilibili API
const QR_WAITING = 86101;
const QR_SCANNED = 86090;
const QR_EXPIRED = 86038;
const QR_CONFIRMED = 0;

interface CookieFields {
  sessdata: string;
  biliJct: string;
  dedeUserId: string;
}

interface QRSession {
  url: string;
  qrcode_key: string;
}

interface BilibiliStatus {
  logged_in: boolean;
  username?: string;
  error?: string;
}

function parseCookie(cookieStr: string): CookieFields {
  const fields: CookieFields = { sessdata: "", biliJct: "", dedeUserId: "" };
  if (!cookieStr) return fields;

  const parts = cookieStr.split(";").map((p) => p.trim());
  for (const part of parts) {
    const [key, ...valueParts] = part.split("=");
    const value = valueParts.join("=");
    if (key === "SESSDATA") fields.sessdata = value;
    else if (key === "bili_jct") fields.biliJct = value;
    else if (key === "DedeUserID") fields.dedeUserId = value;
  }
  return fields;
}

function buildCookie(fields: CookieFields): string {
  const parts: string[] = [];
  if (fields.sessdata) parts.push(`SESSDATA=${fields.sessdata}`);
  if (fields.biliJct) parts.push(`bili_jct=${fields.biliJct}`);
  if (fields.dedeUserId) parts.push(`DedeUserID=${fields.dedeUserId}`);
  return parts.join("; ");
}

export function BilibiliPage() {
  const { isConnected, showToast } = useApp();
  const [loginMethod, setLoginMethod] = useState<LoginMethod>("qr");
  const [status, setStatus] = useState<BilibiliStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/bilibili/status");
      const data = await res.json();
      if (data.code === 200) {
        setStatus(data.data);
      }
    } catch (error) {
      console.error("Failed to fetch status:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch login status on mount
  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const handleLogout = async () => {
    try {
      await setConfigValue("bilibili.cookie", "");
      setStatus({ logged_in: false });
      showToast("success", "已退出登录");
    } catch (error) {
      console.error("Failed to logout:", error);
      showToast("error", "退出失败");
    }
  };

  if (loading) {
    return (
      <div className="max-w-3xl mx-auto p-6">
        <h1 className="text-2xl font-bold mb-6">Bilibili</h1>
        <div className="text-zinc-500">Loading...</div>
      </div>
    );
  }

  // Already logged in view
  if (status?.logged_in) {
    return (
      <div className="max-w-3xl mx-auto p-6">
        <h1 className="text-2xl font-bold mb-6">Bilibili</h1>

        <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-6">
          <div className="flex items-center gap-3 mb-4">
            <span className="inline-block w-3 h-3 rounded-full bg-green-500" />
            <span className="text-lg font-medium">已登录</span>
          </div>

          <div className="mb-6">
            <span className="text-zinc-600 dark:text-zinc-400">用户: </span>
            <span className="font-medium">{status.username || "Unknown"}</span>
          </div>

          <button
            onClick={handleLogout}
            disabled={!isConnected}
            className="px-4 py-2 text-sm font-medium text-red-600 dark:text-red-400 border border-red-300 dark:border-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            退出登录
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto p-6">
      <h1 className="text-2xl font-bold mb-6">Bilibili</h1>

      {/* Status */}
      <div className="mb-6 flex items-center gap-2">
        <span className="inline-block w-2 h-2 rounded-full bg-zinc-400" />
        <span className="text-sm text-zinc-600 dark:text-zinc-400">未登录</span>
      </div>

      {/* Tab Buttons */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setLoginMethod("qr")}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
            loginMethod === "qr"
              ? "bg-blue-600 text-white"
              : "bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200 dark:hover:bg-zinc-700"
          }`}
        >
          扫码登录
        </button>
        <button
          onClick={() => setLoginMethod("cookie")}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
            loginMethod === "cookie"
              ? "bg-blue-600 text-white"
              : "bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 hover:bg-zinc-200 dark:hover:bg-zinc-700"
          }`}
        >
          Cookie 登录
        </button>
      </div>

      {/* Content */}
      {loginMethod === "qr" ? (
        <QRLogin onSuccess={fetchStatus} />
      ) : (
        <CookieLogin onSuccess={fetchStatus} />
      )}
    </div>
  );
}

// QR Code Login Component
function QRLogin({ onSuccess }: { onSuccess: () => void }) {
  const { isConnected, showToast } = useApp();
  const [qrSession, setQrSession] = useState<QRSession | null>(null);
  const [qrStatus, setQrStatus] = useState<number | null>(null);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollIntervalRef = useRef<number | null>(null);

  const generateQR = useCallback(async () => {
    console.log("[Bilibili] Generating QR code...");
    setGenerating(true);
    setError(null);
    setQrStatus(null);

    try {
      const res = await fetch("/api/bilibili/qr/generate", { method: "POST" });
      const data = await res.json();
      console.log("[Bilibili] Generate response:", data);

      if (data.code === 200) {
        console.log("[Bilibili] Setting qrSession and qrStatus to QR_WAITING:", QR_WAITING);
        setQrSession(data.data);
        setQrStatus(QR_WAITING);
      } else {
        setError(data.message || "生成二维码失败");
      }
    } catch (err) {
      console.error("[Bilibili] Generate error:", err);
      setError("网络错误，请重试");
    } finally {
      setGenerating(false);
    }
  }, []);

  const pollStatus = useCallback(async () => {
    if (!qrSession) return;

    try {
      const res = await fetch(
        `/api/bilibili/qr/poll?qrcode_key=${encodeURIComponent(qrSession.qrcode_key)}`
      );
      const data = await res.json();
      console.log("Poll response:", data);

      if (data.code === 200) {
        const status = data.data.status;
        console.log("QR status:", status, "status_text:", data.data.status_text);
        setQrStatus(status);

        if (status === QR_CONFIRMED) {
          // Login successful
          console.log("Login confirmed! Username:", data.data.username);
          if (pollIntervalRef.current) {
            clearInterval(pollIntervalRef.current);
            pollIntervalRef.current = null;
          }
          showToast("success", `登录成功！欢迎，${data.data.username || "用户"}`);
          onSuccess();
        } else if (status === QR_EXPIRED) {
          // QR expired
          if (pollIntervalRef.current) {
            clearInterval(pollIntervalRef.current);
            pollIntervalRef.current = null;
          }
        }
      } else {
        console.error("Poll error response:", data);
      }
    } catch (err) {
      console.error("Poll error:", err);
    }
  }, [qrSession, showToast, onSuccess]);

  // Generate QR on mount
  useEffect(() => {
    if (isConnected) {
      generateQR();
    }
  }, [isConnected, generateQR]);

  // Poll status while waiting or scanned
  useEffect(() => {
    // Only poll when we have a session and status is waiting or scanned
    const shouldPoll =
      qrSession &&
      (qrStatus === QR_WAITING || qrStatus === QR_SCANNED);

    console.log("[Bilibili] Polling useEffect:", {
      hasSession: !!qrSession,
      qrStatus,
      shouldPoll,
      QR_WAITING,
      QR_SCANNED
    });

    if (shouldPoll) {
      // Clear any existing interval first
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
      console.log("[Bilibili] Starting poll interval");
      pollIntervalRef.current = window.setInterval(pollStatus, 1500);
    }

    return () => {
      if (pollIntervalRef.current) {
        console.log("[Bilibili] Clearing poll interval");
        clearInterval(pollIntervalRef.current);
        pollIntervalRef.current = null;
      }
    };
  }, [qrSession, qrStatus, pollStatus]);

  const getStatusText = () => {
    switch (qrStatus) {
      case QR_WAITING:
        return "请使用 Bilibili 客户端扫描二维码";
      case QR_SCANNED:
        return "扫码成功！请在手机上确认登录";
      case QR_EXPIRED:
        return "二维码已过期";
      case QR_CONFIRMED:
        return "登录成功！";
      default:
        return "";
    }
  };

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-6">
      <h3 className="text-lg font-medium mb-4">扫码登录</h3>

      <div className="flex flex-col items-center">
        {/* QR Code Display */}
        <div className="mb-4 p-4 bg-white rounded-lg">
          {generating ? (
            <div className="w-48 h-48 flex items-center justify-center text-zinc-500">
              生成中...
            </div>
          ) : error ? (
            <div className="w-48 h-48 flex items-center justify-center text-red-500 text-center text-sm">
              {error}
            </div>
          ) : qrSession ? (
            <QRCodeSVG
              value={qrSession.url}
              size={192}
              level="L"
              className={qrStatus === QR_EXPIRED ? "opacity-30" : ""}
            />
          ) : (
            <div className="w-48 h-48 flex items-center justify-center text-zinc-500">
              等待生成...
            </div>
          )}
        </div>

        {/* Status Text */}
        <div className="mb-4 text-center">
          {qrStatus === QR_SCANNED ? (
            <span className="text-green-600 dark:text-green-400 font-medium">
              {getStatusText()}
            </span>
          ) : qrStatus === QR_EXPIRED ? (
            <span className="text-red-600 dark:text-red-400">
              {getStatusText()}
            </span>
          ) : (
            <span className="text-zinc-600 dark:text-zinc-400">
              {getStatusText()}
            </span>
          )}
        </div>

        {/* Refresh Button */}
        {(qrStatus === QR_EXPIRED || error) && (
          <button
            onClick={generateQR}
            disabled={!isConnected || generating}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {generating ? "..." : "重新生成"}
          </button>
        )}
      </div>
    </div>
  );
}

// Cookie Login Component
function CookieLogin({ onSuccess }: { onSuccess: () => void }) {
  const { isConnected, showToast } = useApp();
  const [fields, setFields] = useState<CookieFields>({
    sessdata: "",
    biliJct: "",
    dedeUserId: "",
  });
  const [saving, setSaving] = useState(false);

  // Load existing cookie on mount
  useEffect(() => {
    fetch("/api/config")
      .then((res) => res.json())
      .then((data) => {
        if (data.data?.bilibili_cookie) {
          setFields(parseCookie(data.data.bilibili_cookie));
        }
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    const cookie = buildCookie(fields);
    if (!cookie) return;

    setSaving(true);
    try {
      await setConfigValue("bilibili.cookie", cookie);
      showToast("success", "登录成功！前往首页开始下载 Bilibili 视频");
      onSuccess();
    } catch (error) {
      console.error("Failed to save cookie:", error);
      showToast("error", "保存失败，请重试");
    } finally {
      setSaving(false);
    }
  };

  const hasAnyInput = fields.sessdata || fields.biliJct || fields.dedeUserId;

  return (
    <div className="bg-white dark:bg-zinc-800 rounded-lg border border-zinc-200 dark:border-zinc-700 p-6">
      {/* Instructions */}
      <div className="mb-6 p-4 bg-zinc-50 dark:bg-zinc-900 rounded-lg">
        <h3 className="text-sm font-medium mb-3">获取 Cookie 的方法</h3>
        <ol className="space-y-2 text-sm text-zinc-600 dark:text-zinc-400">
          <li>1. 在浏览器中打开 bilibili.com 并登录</li>
          <li>2. 按 F12 打开开发者工具</li>
          <li>3. 切换到「应用」(Application) 标签</li>
          <li>4. 在左侧找到 Cookies → bilibili.com</li>
          <li>5. 复制以下字段的值，粘贴到下方输入框</li>
        </ol>
      </div>

      {/* Cookie Input */}
      <div className="space-y-4">
        {/* SESSDATA */}
        <div>
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
            SESSDATA
          </label>
          <input
            type="text"
            value={fields.sessdata}
            onChange={(e) =>
              setFields((f) => ({ ...f, sessdata: e.target.value }))
            }
            placeholder="粘贴 SESSDATA 值"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
            disabled={!isConnected || saving}
          />
        </div>

        {/* bili_jct */}
        <div>
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
            bili_jct
          </label>
          <input
            type="text"
            value={fields.biliJct}
            onChange={(e) =>
              setFields((f) => ({ ...f, biliJct: e.target.value }))
            }
            placeholder="粘贴 bili_jct 值"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
            disabled={!isConnected || saving}
          />
        </div>

        {/* DedeUserID */}
        <div>
          <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">
            DedeUserID
          </label>
          <input
            type="text"
            value={fields.dedeUserId}
            onChange={(e) =>
              setFields((f) => ({ ...f, dedeUserId: e.target.value }))
            }
            placeholder="粘贴 DedeUserID 值"
            className="w-full px-3 py-2 text-sm bg-zinc-50 dark:bg-zinc-900 border border-zinc-300 dark:border-zinc-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
            disabled={!isConnected || saving}
          />
        </div>

        {/* Save Button */}
        <button
          onClick={handleSave}
          disabled={!isConnected || saving || !hasAnyInput}
          className="w-full px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {saving ? "保存中..." : "保存"}
        </button>
      </div>
    </div>
  );
}
